package process

import (
	"context"
	"errors"
	"fmt"

	"github.com/taosdata/taoskeeper/db"

	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	taosError "github.com/taosdata/driver-go/v3/errors"

	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
	"github.com/taosdata/taoskeeper/util/pool"
)

var logger = log.GetLogger("handle")

type CollectType string

const (
	Counter CollectType = "counter"
	Gauge   CollectType = "gauge"
	Info    CollectType = "info"
	Summary CollectType = "summary"
)

type Processor struct {
	prefix           string
	db               string
	metrics          map[string]*Table //tableName:*Table{}
	tableList        []string
	ctx              context.Context
	rotationInterval time.Duration
	nextTime         time.Time
	exitChan         chan struct{}
	dbConn           *db.Connector
	summaryTable     map[string]*Table
	tables           map[string]struct{}
}

func (p *Processor) Describe(descs chan<- *prometheus.Desc) {
	for _, tableName := range p.tableList {
		table := p.metrics[tableName]
		for _, metric := range table.Metrics {
			descs <- metric.Desc
		}
	}
}

func (p *Processor) Collect(metrics chan<- prometheus.Metric) {
	for _, tableName := range p.tableList {
		table := p.metrics[tableName]
		for _, metric := range table.NewMetrics {
			switch metric.Type {
			case Gauge:
				gv := prometheus.NewGaugeVec(prometheus.GaugeOpts{
					Name:        metric.FQName,
					Help:        metric.Help,
					ConstLabels: metric.ConstLabels,
				}, table.Variables)
				for _, value := range metric.GetValue() {
					if value.Value == nil {
						continue
					}
					g := gv.With(value.Label)
					g.Set(value.Value.(float64))
					metrics <- g
				}
			case Counter:
				cv := prometheus.NewCounterVec(prometheus.CounterOpts{
					Name:        metric.FQName,
					Help:        metric.Help,
					ConstLabels: metric.ConstLabels,
				}, table.Variables)
				for _, value := range metric.GetValue() {
					if value.Value == nil {
						continue
					}
					v := i2float(value.Value)
					if v < 0 {
						logger.Warningf("negative value for prometheus counter. label %v value %v",
							value.Label, value.Value)
						continue
					}
					c := cv.With(value.Label)
					c.Add(v)
					metrics <- c
				}
			case Info:
				lbs := []string{"value"}
				lbs = append(lbs, table.Variables...)
				gf := prometheus.NewGaugeVec(prometheus.GaugeOpts{
					Name:        metric.FQName,
					Help:        metric.Help,
					ConstLabels: metric.ConstLabels,
				}, lbs)
				for _, value := range metric.GetValue() {
					if value == nil {
						continue
					}
					v := make(map[string]string, len(value.Label)+1)
					v["value"] = value.Value.(string)
					for k, l := range value.Label {
						v[k] = l
					}
					g := gf.With(v)
					g.Set(1)
					metrics <- g
				}
			case Summary:
			}
		}
	}
}

type Table struct {
	Variables  []string
	Metrics    []*Metric
	NewMetrics map[string]*Metric // column name -> Metric
	ColumnList []string
}

type Metric struct {
	sync.RWMutex
	FQName      string
	Help        string
	Type        CollectType
	ColType     int
	ConstLabels map[string]string
	Desc        *prometheus.Desc
	LastValue   []*Value
}

func (m *Metric) SetValue(v []*Value) {
	m.Lock()
	defer m.Unlock()
	m.LastValue = v
}

func (m *Metric) GetValue() []*Value {
	m.RLock()
	defer m.RUnlock()
	return m.LastValue
}

type Value struct {
	Label map[string]string
	Value interface{}
}

func NewProcessor(conf *config.Config) *Processor {
	conn, err := db.NewConnector(conf.TDengine.Username, conf.TDengine.Password, conf.TDengine.Host, conf.TDengine.Port)
	if err != nil {
		panic(err)
	}
	interval, err := time.ParseDuration(conf.RotationInterval)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	tables, err := ExpandMetricsFromConfig(ctx, conn, &conf.Metrics)
	if err != nil {
		panic(err)
	}
	p := &Processor{
		prefix:           conf.Metrics.Prefix,
		db:               conf.Metrics.Database,
		metrics:          map[string]*Table{},
		ctx:              ctx,
		rotationInterval: interval,
		exitChan:         make(chan struct{}),
		dbConn:           conn,
		summaryTable:     map[string]*Table{"taosadapter_restful_http_request_summary_milliseconds": nil},
		tables:           tables,
	}
	p.Prepare()
	p.process()
	p.setNextTime(time.Now())
	go p.work()
	return p
}

func (p *Processor) Prepare() {
	locker := sync.RWMutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(p.tables))

	for tn := range p.tables {
		tableName := tn

		err := pool.GoroutinePool.Submit(func() {
			defer wg.Done()
			data, err := p.dbConn.Query(p.ctx, fmt.Sprintf("describe %s", p.withDBName(tableName)))
			if err != nil {
				var tdEngineError *taosError.TaosError
				if errors.As(err, &tdEngineError) {
					logger.WithError(err).Errorf("table %s not exist, skip", tableName)
				} else {
					logger.WithError(err).Errorf("could not get table %s metadata, skip", tableName)
				}
				return
			}

			tags := make([]string, 0, len(data.Data))
			columns := make([]string, 0, len(data.Data))
			typeList := make([]string, 0, len(data.Data))
			columnMap := make(map[string]struct{}, len(data.Data))
			variablesMap := make(map[string]struct{}, len(data.Data))
			for _, info := range data.Data {
				if info[3].(string) != "" {
					variable := info[0].(string)
					tags = append(tags, variable)
					variablesMap[variable] = struct{}{}
				} else {
					column := info[0].(string)
					columns = append(columns, column)
					typeList = append(typeList, info[1].(string))
					columnMap[column] = struct{}{}
				}
			}

			metrics := make([]*Metric, 0, len(columns))
			newMetrics := make(map[string]*Metric, len(columns))
			columnList := make([]string, 0, len(columns))

			_, exist := p.summaryTable[tableName]
			for i, column := range columns {
				if _, columnExist := variablesMap[column]; columnExist {
					continue
				}

				if typeList[i] == "TIMESTAMP" {
					continue
				}

				columnName, metricType := "", Summary
				if !exist {
					columnName = column
					metricType = exchangeDBType(typeList[i])
				}

				labels := make(map[string]string)
				fqName := p.buildFQName(tableName, "", columnName, "")
				pDesc := prometheus.NewDesc(fqName, "", nil, labels)
				metric := &Metric{
					Type:        metricType,
					Desc:        pDesc,
					FQName:      fqName,
					Help:        "",
					ConstLabels: labels,
				}
				metrics = append(metrics, metric)
				newMetrics[column] = metric
				columnList = append(columnList, column)
			}

			t := &Table{
				Variables:  tags,
				Metrics:    metrics,
				NewMetrics: newMetrics,
				ColumnList: columnList,
			}
			locker.Lock()
			p.metrics[tableName] = t
			p.tableList = append(p.tableList, tableName)
			locker.Unlock()
		})
		if err != nil {
			panic(err)
		}
	}

	wg.Wait()
}

func (p *Processor) withDBName(tableName string) string {
	b := pool.BytesPoolGet()
	b.WriteString(p.db)
	b.WriteByte('.')
	b.WriteString(tableName)
	return b.String()
}

func (p *Processor) setNextTime(t time.Time) {
	p.nextTime = t.Round(p.rotationInterval)
	if p.nextTime.Before(time.Now()) {
		p.nextTime = p.nextTime.Add(p.rotationInterval)
	}
}

func (p *Processor) work() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case t := <-ticker.C:
			if t.After(p.nextTime) {
				p.process()
				p.setNextTime(time.Now())
			}
		case <-p.exitChan:
			logger.Warn("exit process")
			return
		}
	}
}

func (p *Processor) process() {
	for _, tableName := range p.tableList {
		tagIndex := 0
		hasTag := false
		b := pool.BytesPoolGet()
		b.WriteString("select ")

		table := p.metrics[tableName]
		columns := table.ColumnList
		for i, column := range columns {
			b.WriteString("last_row(`" + column + "`) as `" + column + "`")
			if i != len(columns)-1 {
				b.WriteByte(',')
			}
		}

		if len(table.Variables) > 0 {
			tagIndex = len(columns)
			for _, tag := range table.Variables {
				b.WriteString(", last_row(`" + tag + "`) as `" + tag + "`")
			}
		}

		b.WriteString(" from ")
		b.WriteString(p.withDBName(tableName))
		if len(table.Variables) > 0 {
			tagIndex = len(columns)
			b.WriteString(" group by ")
			for i, tag := range table.Variables {
				b.WriteString("`" + tag + "`")
				if i != len(table.Variables)-1 {
					b.WriteByte(',')
				}
			}
		}
		sql := b.String()
		pool.BytesPoolPut(b)
		data, err := p.dbConn.Query(p.ctx, sql)
		logger.Debug(sql)
		if err != nil {
			logger.WithError(err).Errorln("select data sql:", sql)
			continue
		}
		if tagIndex > 0 {
			hasTag = true
		}
		if len(data.Data) == 0 {
			continue
		}
		values := make([][]*Value, len(table.ColumnList))
		for _, row := range data.Data {
			label := map[string]string{}
			valuesMap := make(map[string]interface{})
			colEndIndex := len(columns)
			if hasTag {
				for i := tagIndex; i < len(data.Head); i++ {
					if row[i] != nil {
						label[data.Head[i]] = fmt.Sprintf("%v", row[i])
					}
				}
			}
			// values array to map
			for i := 0; i < colEndIndex; i++ {
				valuesMap[columns[i]] = row[i]
			}
			for i, column := range table.ColumnList {
				var v interface{}
				metric := table.NewMetrics[column]
				switch metric.Type {
				case Info:
					if valuesMap[column] != nil {
						v = i2string(valuesMap[column])
					} else {
						v = nil
					}
				case Counter, Gauge, Summary:
					if valuesMap[column] != nil {
						v = i2float(valuesMap[column])
					} else {
						v = nil
					}
				}
				values[i] = append(values[i], &Value{
					Label: label,
					Value: v,
				})
			}
		}
		for i, column := range table.ColumnList {
			metric := table.NewMetrics[column]
			logger.Debugf("set metric [%d] value as %v", i, values[i])
			metric.SetValue(values[i])
		}
	}
}

func (p *Processor) buildFQName(tableName, alias, column, unit string) string {
	b := pool.BytesPoolGet()
	b.WriteString(p.prefix)
	b.WriteByte('_')
	if alias != "" {
		b.WriteString(alias)
	} else {
		tempTableName := strings.TrimPrefix(tableName, "taosd_")
		tempTableName = strings.TrimPrefix(tempTableName, "taos_")
		b.WriteString(tempTableName)
	}
	if column != "" {
		b.WriteByte('_')
		b.WriteString(column)
	}

	if len(unit) != 0 {
		b.WriteByte('_')
		b.WriteString(unit)
	}
	fqName := b.String()
	pool.BytesPoolPut(b)
	return fqName
}

func (p *Processor) Close() error {
	close(p.exitChan)
	return p.dbConn.Close()
}

func (p *Processor) GetMetric() map[string]*Table {
	return p.metrics
}

func exchangeDBType(t string) CollectType {
	switch t {
	case "BOOL", "FLOAT", "DOUBLE":
		return Gauge
	case "TINYINT", "SMALLINT", "INT", "BIGINT", "TINYINT UNSIGNED", "SMALLINT UNSIGNED", "INT UNSIGNED", "BIGINT UNSIGNED":
		return Counter
	case "BINARY", "NCHAR", "VARCHAR":
		return Info
	default:
		panic("unsupported type")
	}
}

func i2string(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		panic("unexpected type to string")
	}
}

func i2float(value interface{}) float64 {
	switch v := value.(type) {
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	case bool:
		if v {
			return 1
		}
		return 0
	default:
		panic("unexpected type to float64")
	}
}
