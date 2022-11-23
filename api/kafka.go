package api

import (
	"context"
	"fmt"
	"github.com/taosdata/go-utils/json"
	"github.com/taosdata/taoskeeper/db"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/taosdata/taoskeeper/infrastructure/config"
)

var kafkaSqls = []string{
	KafkaConnectSql,
	KafkaTaskSql,
}

type KafkaImporter struct {
	pullInterval time.Duration
	nextTime     time.Time
	exitChan     chan struct{}
	client       *http.Client
	username     string
	password     string
	host         string
	port         int
	database     string
	url          string
}

func NewKafkaImporter(conf *config.Config) {
	interval, err := time.ParseDuration(conf.RotationInterval)
	if err != nil {
		panic(err)
	}
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DisableCompression:    true,
		},
	}
	imp := &KafkaImporter{
		pullInterval: interval,
		exitChan:     make(chan struct{}),
		client:       client,
		username:     conf.TDengine.Username,
		password:     conf.TDengine.Password,
		host:         conf.TDengine.Host,
		port:         conf.TDengine.Port,
		database:     conf.Metrics.Database,
		url:          conf.Kafka.Url,
	}

	imp.setNextTime(time.Now())
	go imp.work()
}

func (imp *KafkaImporter) createTable() {
	conn, err := db.NewConnectorWithDb(imp.username, imp.password, imp.host, imp.port, imp.database)
	if err != nil {
		logger.WithError(err).Errorf("connect to database error")
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.WithError(err).Errorf("close connection error")
		}
	}()

	ctx := context.Background()
	for i := 0; i < len(kafkaSqls); i++ {
		logger.Infof("execute sql: %s", kafkaSqls[i])
		if _, err = conn.Exec(ctx, kafkaSqls[i]); err != nil {
			logger.Errorf("execute sql: %s, error: %s", kafkaSqls[i], err)
		}
	}
}

func (imp *KafkaImporter) work() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case t := <-ticker.C:
			if t.After(imp.nextTime) {
				imp.connect()
				imp.setNextTime(time.Now())
			}
		case <-imp.exitChan:
			logger.Warn("exit process")
			return
		}
	}
}

func (imp *KafkaImporter) setNextTime(t time.Time) {
	imp.nextTime = t.Round(imp.pullInterval)
	if imp.nextTime.Before(time.Now()) {
		imp.nextTime = imp.nextTime.Add(imp.pullInterval)
	}
}

func (imp *KafkaImporter) connect() {
	c := imp.query("/")
	var cluster KafkaCluster
	if e := json.Unmarshal(c, &cluster); e != nil {
		logger.WithError(e).Errorf("error occurred while unmarshal request data: %s ", c)
		return
	}

	s := imp.query("/connectors?expand=status")
	var status map[string]KafkaOverview
	if e := json.Unmarshal(s, &status); e != nil {
		logger.WithError(e).Errorf("error occurred while unmarshal request data: %s ", status)
		return
	}

	conn, err := db.NewConnectorWithDb(imp.username, imp.password, imp.host, imp.port, imp.database)
	if err != nil {
		logger.WithError(err).Errorf("connect to database error")
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.WithError(err).Errorf("close connection error")
		}
	}()

	for _, v := range status {
		t := imp.query(fmt.Sprintf("/connectors/%s/topics", v.Status.Name))
		var topics map[string]KafkaTopics
		if e := json.Unmarshal(t, &topics); e != nil {
			logger.WithError(e).Errorf("error occurred while unmarshal request data: %s ", topics)
			continue
		}
		var tstring string
		for _, v := range topics {
			tstring = strings.Join(v.Topics, ",")
		}

		ctx := context.Background()
		sql := fmt.Sprintf("insert into kafka_connect_%s using kafka_connect tags ('%s', '%s') values (now, '%s', '%s', %d, '%s', '%s')",
			v.Status.Name, cluster.ClusterId, v.Status.Name,
			v.Status.Connector.State, v.Status.Connector.WorkerId, len(v.Status.Tasks), v.Status.Type, tstring)
		logger.Tracef("execute sql %s", sql)
		if _, err := conn.Exec(ctx, sql); err != nil {
			logger.WithError(err).Errorf("execute sql : %s", sql)
		}

		for _, task := range v.Status.Tasks {
			sql = fmt.Sprintf("insert into kafka_task_%s using kafka_task tags ('%s', '%s', %d) values (now, '%s', '%s')",
				v.Status.Name, cluster.ClusterId, v.Status.Name, task.Id, task.State, task.WorkerId)
			logger.Tracef("execute sql %s", sql)
			if _, err := conn.Exec(ctx, sql); err != nil {
				logger.WithError(err).Errorf("execute sql : %s", sql)
			}
		}

		//fmt.Sprintf("insert into d_info_%s using d_info tags (%d, '%s', '%s') values ('%s', '%s')",
		//	ClusterID+strconv.Itoa(dnode.DnodeID), dnode.DnodeID, dnode.DnodeEp, ClusterID, ts, dnode.Status))
	}
}

func (imp *KafkaImporter) query(path string) []byte {
	urlPath := &url.URL{
		Scheme: "http",
		Host:   imp.url,
		Path:   path,
	}
	header := map[string][]string{
		"Connection":   {"keep-alive"},
		"Content-Type": {"application/json"},
	}

	req := &http.Request{
		Method:     http.MethodGet,
		URL:        urlPath,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     header,
		Host:       urlPath.Host,
	}

	resp, err := imp.client.Do(req)
	if err != nil {
		logger.Errorf("query metrics from: %s, error: %s", imp.url, err)
	}
	if resp.StatusCode != http.StatusOK {
		_, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Errorf("query metrics status abnormal %d from: %s, error: %s", resp.StatusCode, imp.url, err)
		}
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("error reading body: %s", err)
	}
	return body
}
