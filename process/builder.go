package process

import (
	"context"
	"fmt"

	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
)

var builderLogger = log.GetLogger("builder")

func ExpandMetricsFromConfig(ctx context.Context, conn *db.Connector, cfg *config.MetricsConfig) (tables map[string]struct{}, err error) {
	tables = make(map[string]struct{})
	for _, name := range cfg.Tables {
		builderLogger.Debug("normal table: ", name)

		_, exist := tables[name]
		if exist {
			builderLogger.Debug(name, "is exist in config")
			continue
		}
		tables[name] = struct{}{}
	}

	data, err := conn.Query(ctx, fmt.Sprintf("show %s.stables", cfg.Database))
	if err != nil {
		return nil, err
	}
	builderLogger.Debug("show stables")

	for _, info := range data.Data {
		name := info[0].(string)
		builderLogger.Debug("stable: ", info)

		_, exist := tables[name]
		if exist {
			builderLogger.Debug(name, "is exist in config")
			continue
		}
		tables[name] = struct{}{}
	}
	return
}
