package process

import (
	"context"
	"fmt"

	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
)

var builderLogger = log.GetLogger("builder")

func ExpandMetricsFromConfig(ctx context.Context, conn *db.Connector, cfg *config.MetricsConfig) error {

	for _, name := range cfg.Normals {
		builderLogger.Debug("normal table: ", name)

		_, exist := cfg.Tables[name]
		if exist {
			builderLogger.Debug(name, "is exist in config")
			continue
		}
		cfg.Tables[name] = struct{}{}
	}

	data, err := conn.Query(ctx, fmt.Sprintf("show %s.stables", cfg.Database))
	if err != nil {
		return err
	}
	builderLogger.Debug("show stables")

	for _, info := range data.Data {
		name := info[0].(string)
		builderLogger.Debug("stable: ", info)

		_, exist := cfg.Tables[name]
		if exist {
			builderLogger.Debug(name, "is exist in config")
			continue
		}
		cfg.Tables[name] = struct{}{}
	}
	return err
}
