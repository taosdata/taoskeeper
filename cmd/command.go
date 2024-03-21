package cmd

import (
	"context"

	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
)

var logger = log.GetLogger("command")

func Process(conf *config.Config) {
	if len(conf.Transfer) > 0 && conf.Transfer != "old_taosd_metric" {
		logger.Errorf("transfer only support old_taosd_metric")
		return
	}

	if conf.Transfer == "old_taosd_metric" {
		ProcessTransfer(conf)
		return
	}

	if len(conf.Drop) > 0 && conf.Drop != "old_taosd_metric_stables" {
		logger.Errorf("drop only support old_taosd_metric_stables")
		return
	}

	if conf.Drop == "old_taosd_metric_stables" {
		ProcessDrop(conf)
		return
	}
}

func ProcessTransfer(Conf *config.Config) {

	return
}

func ProcessDrop(conf *config.Config) {
	conn, err := db.NewConnectorWithDb(conf.TDengine.Username, conf.TDengine.Password, conf.TDengine.Host, conf.TDengine.Port, conf.Metrics.Database)
	if err != nil {
		logger.Error("## init db connect error", "error", err)
		return
	}
	var dropStableList = []string{
		"log_dir",
		"dnodes_info",
		"data_dir",
		"log_summary",
		"m_info",
		"vnodes_role",
		"cluster_info",
		"temp_dir",
		"grants_info",
		"vgroups_info",
		"d_info",
		"taosadapter_system_cpu_percent",
		"taosadapter_restful_http_request_in_flight",
		"taosadapter_restful_http_request_summary_milliseconds",
		"taosadapter_restful_http_request_fail",
		"taosadapter_system_mem_percent",
		"taosadapter_restful_http_request_total",
	}
	ctx := context.Background()

	for _, stable := range dropStableList {
		if _, err = conn.Exec(ctx, "drop stable if exist "+stable); err != nil {
			logger.Errorf("drop stable %s, error: %s", stable, err)
		}
	}
	logger.Info("## drop old taosd metric stables success!!")
}
