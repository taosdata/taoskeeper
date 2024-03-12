package monitor

import (
	"github.com/BurntSushi/toml"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	conf, err := getConfig()
	if err != nil {
		panic(err)
	}
	interval, err := time.ParseDuration(conf.RotationInterval)
	if err != nil {
		panic(err)
	}
	Start(interval, conf.Env.InCGroup)
}

func getConfig() (config.Config, error) {
	data := `
# Start with debug middleware for gin
debug = false

# Listen port, default is 6043
port = 6043

# log level
loglevel = "info"

# go pool size
gopoolsize = 50000

# interval for metrics
RotationInterval = "5s"

[tdengine]
host = "127.0.0.1"
port = 6041
username = "root"
password = "taosdata"

[metrics]
# metrics prefix in metrics names.
prefix = "taos"

# database for storing metrics data
database = "log"

# export some tables that are not super table
tables = []

# database options for db storing metrics data
[metrics.databaseoptions]
cachemodel = "none"

[environment]
# Whether running in cgroup.
incgroup = true

[audit]
[audit.database]
name = "audit"
[audit.database.options]
cachemodel = "none"

[log]
#path = "/var/log/taos"
rotationCount = 5
rotationTime = "24h"
rotationSize = 100000000
`
	var c config.Config
	_, err := toml.Decode(data, &c)
	return c, err
}
