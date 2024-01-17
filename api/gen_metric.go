package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
)

var gmLogger = log.GetLogger("gen_metric")

type GeneralMetric struct {
	client   *http.Client
	username string
	password string
	host     string
	port     int
	database string
	url      *url.URL
}

type Tag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Metric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type MetricGroup struct {
	Tags    []Tag    `json:"tags"`
	Metrics []Metric `json:"metrics"`
}

type StableInfo struct {
	Name         string        `json:"name"`
	MetricGroups []MetricGroup `json:"metric_groups"`
}

type StableArrayInfo struct {
	Ts       string       `json:"ts"`
	Protocol int          `json:"protocol"`
	Tables   []StableInfo `json:"tables"`
}

func (a *GeneralMetric) Init(c gin.IRouter) error {
	c.POST("/general-metric", a.handleFunc())
	c.POST("/taosd-cluster-basic", a.handlTaosdClusterBasic())
	return nil
}

func NewGeneralMetric(conf *config.Config) *GeneralMetric {
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

	imp := &GeneralMetric{
		client:   client,
		username: conf.TDengine.Username,
		password: conf.TDengine.Password,
		host:     conf.TDengine.Host,
		port:     conf.TDengine.Port,
		database: conf.Metrics.Database,
		url: &url.URL{
			Scheme:   "http",
			Host:     fmt.Sprintf("%s:%d", conf.TDengine.Host, conf.TDengine.Port),
			Path:     "/influxdb/v1/write",
			RawQuery: fmt.Sprintf("db=%s", conf.Metrics.Database),
		},
	}
	return imp
}

func (imp *GeneralMetric) handleFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		if imp.client == nil {
			gmLogger.Error("no connection")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no connection"})
			return
		}

		data, err := c.GetRawData()
		if err != nil {
			gmLogger.WithError(err).Errorf("## get general metric data error")
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("get general metric data error. %s", err)})
			return
		}
		gmLogger.Trace("## receive general metric data", "data", data)

		var request []StableArrayInfo

		if err := json.Unmarshal(data, &request); err != nil {
			gmLogger.WithError(err).Errorf("## parse general metric data %s error", string(data))
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse general metric data error: %s", err)})
			return
		}

		if len(request) == 0 {
			c.JSON(http.StatusOK, gin.H{})
			return
		}

		err = imp.handleBatchMetrics(request)

		if err != nil {
			gmLogger.WithError(err).Errorf("## process records error")
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("process records error. %s", err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{})
	}
}

func (imp *GeneralMetric) handleBatchMetrics(request []StableArrayInfo) error {
	var builder strings.Builder

	for _, stableArrayInfo := range request {
		if stableArrayInfo.Ts == "" {
			// log err
			continue
		}

		for _, table := range stableArrayInfo.Tables {
			if table.Name == "" {
				// log err
				continue
			}

			for _, metricGroup := range table.MetricGroups {
				builder.WriteString(table.Name)

				for _, tag := range metricGroup.Tags {
					line := fmt.Sprintf(",%s=%s", tag.Name, tag.Value)
					builder.WriteString(line)
				}
				builder.WriteString(" ")

				for i, metric := range metricGroup.Metrics {
					line := fmt.Sprintf("%s=%ff64", metric.Name, metric.Value)
					builder.WriteString(line)
					if i != len(metricGroup.Metrics)-1 {
						builder.WriteString(",")
					}
				}

				builder.WriteString(" ")
				builder.WriteString(stableArrayInfo.Ts)
				builder.WriteString("\n")
			}
		}
	}

	return imp.lineWriteBody(builder.String())
}

func (imp *GeneralMetric) lineWriteBody(data string) error {
	header := map[string][]string{
		"Connection": {"keep-alive"},
	}
	req := &http.Request{
		Method:     http.MethodPost,
		URL:        imp.url,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     header,
		Host:       imp.url.Host,
	}
	req.SetBasicAuth(imp.username, imp.password)

	req.Body = io.NopCloser(strings.NewReader(data))
	resp, err := imp.client.Do(req)

	if err != nil {
		gmLogger.Errorf("writing metrics exception: %v", err)
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d:body:%s", resp.StatusCode, string(body))
	}
	return nil
}

func (imp *GeneralMetric) handlTaosdClusterBasic() gin.HandlerFunc {
	return nil
}
