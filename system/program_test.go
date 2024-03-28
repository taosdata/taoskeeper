package system

import (
	"github.com/taosdata/go-utils/web"
	"github.com/taosdata/taoskeeper/api"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
	"github.com/taosdata/taoskeeper/monitor"
	"github.com/taosdata/taoskeeper/process"
	"github.com/taosdata/taoskeeper/version"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	conf := config.InitConfig()
	conf.Metrics.Database = "program"
	log.ConfigLog()
	router := web.CreateRouter(conf.Debug, &conf.Cors, false)

	reporter := api.NewReporter(conf)
	reporter.Init(router)
	monitor.StartMonitor("", conf, reporter)
	go func() {
		// wait for monitor to all metric received
		time.Sleep(time.Second * 35)

		processor := process.NewProcessor(conf)
		node := api.NewNodeExporter(processor)
		node.Init(router)
	}()

	checkHealth := api.NewCheckHealth(version.Version)
	checkHealth.Init(router)
	audit, err := api.NewAudit(conf)
	if err != nil {
		panic(err)
	}
	if err = audit.Init(router); err != nil {
		panic(err)
	}
	adapter := api.NewAdapter(conf)
	if err = adapter.Init(router); err != nil {
		panic(err)
	}

	gen_metric := api.NewGeneralMetric(conf)
	if err = gen_metric.Init(router); err != nil {
		panic(err)
	}

}
