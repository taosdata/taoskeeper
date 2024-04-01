package system

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"testing"
)

func TestStart(t *testing.T) {
	conf := config.InitConfig()
	server := Init()
	assert.NotNil(t, server)

	conn, err := db.NewConnectorWithDb(conf.TDengine.Username, conf.TDengine.Password, conf.TDengine.Host, conf.TDengine.Port, conf.Metrics.Database)
	assert.NoError(t, err)
	conn.Query(context.Background(), fmt.Sprintf("drop database if exists %s", conf.Metrics.Database))
	conn.Query(context.Background(), fmt.Sprintf("drop database if exists %s", conf.Audit.Database))
}
