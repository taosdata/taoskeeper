package api

import (
	"bytes"
	"fmt"
	tmetric "github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/plugins/inputs/prometheus"
	"github.com/influxdata/telegraf/plugins/serializers/influx"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AdapterImporter struct {
	pullInterval time.Duration
	nextTime     time.Time
	exitChan     chan struct{}
	client       *http.Client
}

func NewAdapterImporter() {
	interval, err := time.ParseDuration(config.Conf.RotationInterval)
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
	imp := &AdapterImporter{
		pullInterval: interval,
		exitChan:     make(chan struct{}),
		client:       client,
	}
	imp.setNextTime(time.Now())
	go imp.work()
}

func (imp *AdapterImporter) work() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case t := <-ticker.C:
			if t.After(imp.nextTime) {
				imp.queryMetrics()
				imp.setNextTime(time.Now())
			}
		case <-imp.exitChan:
			logger.Warn("exit process")
			return
		}
	}
}

func (imp *AdapterImporter) setNextTime(t time.Time) {
	imp.nextTime = t.Round(imp.pullInterval)
	if imp.nextTime.Before(time.Now()) {
		imp.nextTime = imp.nextTime.Add(imp.pullInterval)
	}
}

func (imp *AdapterImporter) queryMetrics() {
	a := *config.Adapter
	for _, addr := range a.Addrs {
		urlPath := &url.URL{
			Scheme: "http",
			Host:   addr,
			Path:   "/metrics",
		}
		header := map[string][]string{
			"Connection": {"keep-alive"},
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
			logger.Errorf("query metrics from: %s, error: %s", addr, err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				logger.Errorf("query metrics status abnormal %d from: %s, error: %s", resp.StatusCode, addr, err)
				continue
			}
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logger.Errorf("error reading body: %s", err)
			continue
		}
		imp.lineWriteBody(body, addr)
		_ = resp.Body.Close()
	}
}

func (imp *AdapterImporter) lineWriteBody(body []byte, addr string) {
	tdConfig := config.Conf.TDengine
	u := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", tdConfig.Host, tdConfig.Port),
		Path:   "/influxdb/v1/write",
		RawQuery: fmt.Sprintf("u=%s&p=%s&db=%s",
			tdConfig.Username, tdConfig.Password, config.Metrics.Database),
	}
	header := map[string][]string{
		"Connection": {"keep-alive"},
	}
	req := &http.Request{
		Method:     http.MethodPost,
		URL:        u,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     header,
		Host:       u.Host,
	}

	metrics, err := prometheus.Parse(body, nil, false)
	if err != nil {
		logger.Errorf("error parse body: %s", err)
		return
	}
	d := bytes.Buffer{}
	for _, metric := range metrics {
		name := metric.Name()
		if !strings.HasPrefix(name, "taosadapter") {
			continue
		}
		tags := metric.Tags()
		tags["endpoint"] = addr
		m := tmetric.New(name, tags, metric.Fields(), metric.Time(), metric.Type())
		data, err := influx.NewSerializer().Serialize(m)
		if err != nil {
			logger.Errorf("error serialize metric: %s, error: %s", metric, err)
			continue
		}
		d.Write(data)
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(d.Bytes()))
	resp, err := imp.client.Do(req)

	if err != nil {
		logger.Errorf("query exception: %v", err)
	}
	_ = resp.Body.Close()
}
