package config

import (
	"flag"
	"fmt"
	"github.com/taosdata/taoskeeper/util/pool"
	"github.com/taosdata/taoskeeper/version"
	"os"
	"strconv"

	// "log"

	"github.com/BurntSushi/toml"
	"github.com/taosdata/go-utils/util"
	"github.com/taosdata/go-utils/web"
	"github.com/taosdata/taoskeeper/infrastructure/log"
)

type Config struct {
	Cors             web.CorsConfig
	Debug            bool
	Port             int
	LogLevel         string
	GoPoolSize       int
	RotationInterval string
	TDengine         TDengineRestful
	TaosAdapter      TaosAdapter
	Metrics          MetricsConfig
	Env              Environment
}

type TDengineRestful struct {
	Host     string
	Port     int
	Username string
	Password string
}

func (conf *TDengineRestful) Init() {
	if val := os.Getenv("TDENGINE_HOST"); val == "" {
		if conf.Host == "" {
			conf.Host = "127.0.0.1"
		}
	} else {
		conf.Host = val
	}
	if val := os.Getenv("TDENGINE_PORT"); val == "" {
		if conf.Port == 0 {
			conf.Port = 6041
		}
	} else {
		port, err := strconv.Atoi(val)
		if err != nil {
			panic(err)
		}
		conf.Port = port
	}
	if val := os.Getenv("TDENGINE_USERNAME"); val == "" {
		if conf.Username == "" {
			conf.Username = "root"
		}
	}
	if val := os.Getenv("TDENGINE_PASSWORD"); val == "" {
		if conf.Password == "" {
			conf.Password = "taosdata"
		}
	} else {
		conf.Password = val
	}
}

func (conf *MetricsConfig) MetricsInit() {
	if conf.Prefix == "" {
		conf.Prefix = "taos"
	}
	if conf.Database == "" {
		conf.Database = "log"
	}
	conf.Tables = make(map[string]struct{})
}

var (
	Conf       *Config
	Metrics    *MetricsConfig
	Adapter    *TaosAdapter
	configPath = "./config/keeper.toml"
)

func Init() {
	cp := flag.String("c", "/etc/taos/keeper.toml", "taoskeeper config file")
	level := flag.String("l", "info", "log level")
	v := flag.Bool("version", false, "Print the version and exit")
	flag.Parse()
	var conf Config
	if *cp != "" {
		configPath = *cp
	}
	if *level != "" {
		conf.LogLevel = *level
	}
	if *v {
		fmt.Printf("%s\n", version.Version)
		os.Exit(0)
	}
	fmt.Println("load config :", configPath)

	log.Init(conf.LogLevel)
	logger := log.GetLogger("config")

	if util.PathExist(configPath) {
		if _, err := toml.DecodeFile(configPath, &conf); err != nil {
			logger.Fatal(err)
		}
	}
	conf.TDengine.Init()
	conf.Cors.Init()
	conf.Metrics.MetricsInit()
	if conf.Port == 0 {
		conf.Port = 6043
	}
	if conf.LogLevel == "" {
		conf.LogLevel = "debug"
	}
	if conf.GoPoolSize == 0 {
		conf.GoPoolSize = 50000
	}
	pool.Init(conf.GoPoolSize)
	if conf.RotationInterval == "" {
		conf.RotationInterval = "15s"
	}
	Conf = &conf
	Metrics = &conf.Metrics
	Adapter = &conf.TaosAdapter
}
