package api

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
)

var adapterLog = log.GetLogger("adapter")

type adapterReqType int

const (
	rest adapterReqType = iota // 0 - rest
	ws                         // 1 - ws
)

type Adapter struct {
	username  string
	password  string
	host      string
	port      int
	usessl    bool
	conn      *db.Connector
	db        string
	dbOptions map[string]interface{}
}

func NewAdapter(c *config.Config) *Adapter {
	return &Adapter{
		username:  c.TDengine.Username,
		password:  c.TDengine.Password,
		host:      c.TDengine.Host,
		port:      c.TDengine.Port,
		usessl:    c.TDengine.Usessl,
		db:        c.Metrics.Database,
		dbOptions: c.Metrics.DatabaseOptions,
	}
}

func (a *Adapter) Init(c gin.IRouter) error {
	if err := a.createDatabase(); err != nil {
		return fmt.Errorf("create database error: %s", err)
	}
	if err := a.initConnect(); err != nil {
		return fmt.Errorf("init db connect error: %s", err)
	}
	if err := a.createTable(); err != nil {
		return fmt.Errorf("create table error: %s", err)
	}
	c.POST("/adapter_report", a.handleFunc())
	return nil
}

func (a *Adapter) handleFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.conn == nil {
			adapterLog.Error("no connection")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no connection"})
			return
		}

		data, err := c.GetRawData()
		if err != nil {
			adapterLog.WithError(err).Errorf("## get adapter report  data error")
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("get adapter report data error. %s", err)})
			return
		}
		adapterLog.Trace("## receive adapter report data", "data", string(data))

		var report AdapterReport
		if err = json.Unmarshal(data, &report); err != nil {
			adapterLog.WithError(err).Errorf("## parse adapter report data %s error", string(data))
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse adapter report data error: %s", err)})
			return
		}
		sql := a.parseSql(report)
		adapterLog.Debug("## adapter report", "sql", sql)

		if _, err = a.conn.Exec(context.Background(), sql); err != nil {
			adapterLog.Error("## adapter report error", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{})
	}
}

func (a *Adapter) initConnect() error {
	conn, err := db.NewConnectorWithDb(a.username, a.password, a.host, a.port, a.db, a.usessl)
	if err != nil {
		adapterLog.Error("## init db connect error", "error", err)
		return err
	}
	a.conn = conn
	return nil
}

func (a *Adapter) parseSql(report AdapterReport) string {
	// reqType: 0: rest, 1: websocket
	restTbName := a.tableName(report.Endpoint, rest)
	wsTbName := a.tableName(report.Endpoint, ws)
	ts := time.Unix(report.Timestamp, 0).Format(time.RFC3339)
	metric := report.Metric
	return fmt.Sprintf("insert into %s using adapter_requests tags ('%s', %d) "+
		"values('%s', %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d) "+
		"%s using adapter_requests tags ('%s', %d) "+
		"values('%s', %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d)",
		restTbName, report.Endpoint, rest, ts, metric.RestTotal, metric.RestQuery, metric.RestWrite, metric.RestOther,
		metric.RestInProcess, metric.RestSuccess, metric.RestFail, metric.RestQuerySuccess, metric.RestQueryFail,
		metric.RestWriteSuccess, metric.RestWriteFail, metric.RestOtherSuccess, metric.RestOtherFail,
		metric.RestQueryInProcess, metric.RestWriteInProcess,
		wsTbName, report.Endpoint, ws, ts, metric.WSTotal,
		metric.WSQuery, metric.WSWrite, metric.WSOther, metric.WSInProcess, metric.WSSuccess, metric.WSFail,
		metric.WSQuerySuccess, metric.WSQueryFail, metric.WSWriteSuccess, metric.WSWriteFail, metric.WSOtherSuccess,
		metric.WSOtherFail, metric.WSQueryInProcess, metric.WSWriteInProcess)
}

func (a *Adapter) tableName(endpoint string, reqType adapterReqType) string {
	sum := md5.Sum([]byte(fmt.Sprintf("%s%d", endpoint, reqType)))
	return fmt.Sprintf("t_%s", hex.EncodeToString(sum[:]))
}

func (a *Adapter) createDatabase() error {
	conn, err := db.NewConnector(a.username, a.password, a.host, a.port, a.usessl)
	if err != nil {
		return fmt.Errorf("connect to database error: %s", err)
	}
	defer func() { _ = conn.Close() }()
	sql := a.createDBSql()
	adapterLog.Info("## create database", "sql", sql)
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		adapterLog.Error("## create database error", "error", err)
		return err
	}

	return err
}

func (a *Adapter) createDBSql() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("create database if not exists %s ", a.db))

	for k, v := range a.dbOptions {
		buf.WriteString(k)
		switch v := v.(type) {
		case string:
			buf.WriteString(fmt.Sprintf(" '%s'", v))
		default:
			buf.WriteString(fmt.Sprintf(" %v", v))
		}
		buf.WriteString(" ")
	}

	return buf.String()
}

var adapterTableSql = "create stable if not exists `adapter_requests` (" +
	"`ts` timestamp, " +
	"`total` int unsigned, " +
	"`query` int unsigned, " +
	"`write` int unsigned, " +
	"`other` int unsigned, " +
	"`in_process` int unsigned, " +
	"`success` int unsigned, " +
	"`fail` int unsigned, " +
	"`query_success` int unsigned, " +
	"`query_fail` int unsigned, " +
	"`write_success` int unsigned, " +
	"`write_fail` int unsigned, " +
	"`other_success` int unsigned, " +
	"`other_fail` int unsigned, " +
	"`query_in_process` int unsigned, " +
	"`write_in_process` int unsigned ) " +
	"tags (`endpoint` varchar(32), `req_type` tinyint unsigned )"

var errNoConnection = errors.New("no connection")

func (a *Adapter) createTable() error {
	if a.conn == nil {
		return errNoConnection
	}
	_, err := a.conn.Exec(context.Background(), adapterTableSql)
	return err
}

type AdapterReport struct {
	Timestamp int64          `json:"ts"`
	Metric    AdapterMetrics `json:"metrics"`
	Endpoint  string         `json:"endpoint"`
}

type AdapterMetrics struct {
	RestTotal          int `json:"rest_total"`
	RestQuery          int `json:"rest_query"`
	RestWrite          int `json:"rest_write"`
	RestOther          int `json:"rest_other"`
	RestInProcess      int `json:"rest_in_process"`
	RestSuccess        int `json:"rest_success"`
	RestFail           int `json:"rest_fail"`
	RestQuerySuccess   int `json:"rest_query_success"`
	RestQueryFail      int `json:"rest_query_fail"`
	RestWriteSuccess   int `json:"rest_write_success"`
	RestWriteFail      int `json:"rest_write_fail"`
	RestOtherSuccess   int `json:"rest_other_success"`
	RestOtherFail      int `json:"rest_other_fail"`
	RestQueryInProcess int `json:"rest_query_in_process"`
	RestWriteInProcess int `json:"rest_write_in_process"`
	WSTotal            int `json:"ws_total"`
	WSQuery            int `json:"ws_query"`
	WSWrite            int `json:"ws_write"`
	WSOther            int `json:"ws_other"`
	WSInProcess        int `json:"ws_in_process"`
	WSSuccess          int `json:"ws_success"`
	WSFail             int `json:"ws_fail"`
	WSQuerySuccess     int `json:"ws_query_success"`
	WSQueryFail        int `json:"ws_query_fail"`
	WSWriteSuccess     int `json:"ws_write_success"`
	WSWriteFail        int `json:"ws_write_fail"`
	WSOtherSuccess     int `json:"ws_other_success"`
	WSOtherFail        int `json:"ws_other_fail"`
	WSQueryInProcess   int `json:"ws_query_in_process"`
	WSWriteInProcess   int `json:"ws_write_in_process"`
}
