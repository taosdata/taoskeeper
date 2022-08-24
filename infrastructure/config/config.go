package config

import (
	"fmt"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/taosdata/taoskeeper/util/pool"
	"github.com/taosdata/taoskeeper/version"
	"os"

	"github.com/taosdata/go-utils/web"
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

var (
	Conf    *Config
	Metrics *MetricsConfig
	Adapter *TaosAdapter
)

func Init() {
	viper.SetConfigType("toml")
	viper.SetConfigName("keeper")
	viper.AddConfigPath("/etc/taos")

	cp := pflag.String("c", "/etc/taos/keeper.toml", "taoskeeper config file")
	v := pflag.Bool("version", false, "Print the version and exit")
	help := pflag.BoolP("help", "h", false, "Print this help message and exit")
	pflag.Parse()

	if *help {
		fmt.Fprintf(os.Stderr, "Usage of taosKeeper v%s:\n", version.Version)
		pflag.PrintDefaults()
		os.Exit(0)
	}

	if *v {
		fmt.Printf("%s\n", version.Version)
		os.Exit(0)
	}
	if *cp != "" {
		viper.SetConfigFile(*cp)
	}

	viper.SetEnvPrefix("taoskeeper")
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		panic(err)
	}
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("config file not found")
		} else {
			panic(err)
		}
	}

	Conf = &Config{
		Debug:            viper.GetBool("debug"),
		Port:             viper.GetInt("port"),
		LogLevel:         viper.GetString("logLevel"),
		GoPoolSize:       viper.GetInt("gopoolsize"),
		RotationInterval: viper.GetString("RotationInterval"),
	}
	Conf.Cors.Init()
	pool.Init(Conf.GoPoolSize)
	Conf.TDengine = TDengineRestful{
		Host:     viper.GetString("tdengine.host"),
		Port:     viper.GetInt("tdengine.port"),
		Username: viper.GetString("tdengine.username"),
		Password: viper.GetString("tdengine.password"),
	}
	Conf.TaosAdapter = TaosAdapter{
		Addrs: viper.GetStringSlice("taosAdapter.address"),
	}

	fmt.Print("TAOS_KEEPER_TAOSADAPTER_ADDRESS = %s", Conf.TaosAdapter.Addrs)
	Conf.Metrics = MetricsConfig{
		Prefix:   viper.GetString("metrics.prefix"),
		Database: viper.GetString("metrics.database"),
		Tables:   map[string]struct{}{},
		Normals:  viper.GetStringSlice("metrics.tables"),
	}
	Conf.Env = Environment{
		InCGroup: viper.GetBool("environment.incgroup"),
	}
	Metrics = &Conf.Metrics
	Adapter = &Conf.TaosAdapter
}

func init() {
	viper.SetDefault("debug", false)
	_ = viper.BindEnv("debug", "TAOS_KEEPER_DEBUG")
	pflag.Bool("debug", false, `enable debug mode. Env "TAOS_KEEPER_DEBUG"`)

	viper.SetDefault("port", 6043)
	_ = viper.BindEnv("port", "TAOS_KEEPER_PORT")
	pflag.IntP("port", "P", 6043, `http port. Env "TAOS_KEEPER_PORT"`)

	viper.SetDefault("logLevel", "info")
	_ = viper.BindEnv("logLevel", "TAOS_KEEPER_LOG_LEVEL")
	pflag.String("logLevel", "info", `log level (panic fatal error warn warning info debug trace). Env "TAOS_KEEPER_LOG_LEVEL"`)

	viper.SetDefault("gopoolsize", 50000)
	_ = viper.BindEnv("gopoolsize", "TAOS_KEEPER_POOL_SIZE")
	pflag.Int("gopoolsize", 50000, `coroutine size. Env "TAOS_KEEPER_POOL_SIZE"`)

	viper.SetDefault("RotationInterval", "15s")
	_ = viper.BindEnv("RotationInterval", "TAOS_KEEPER_ROTATION_INTERVAL")
	pflag.StringP("RotationInterval", "R", "15s", `interval for refresh metrics, such as "300ms", Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h". Env "TAOS_KEEPER_ROTATION_INTERVAL"`)

	viper.SetDefault("tdengine.host", "127.0.0.1")
	_ = viper.BindEnv("tdengine.host", "TAOS_KEEPER_TDENGINE_HOST")
	pflag.String("tdengine.host", "127.0.0.1", `TDengine server's ip. Env "TAOS_KEEPER_TDENGINE_HOST"`)

	viper.SetDefault("tdengine.port", 6041)
	_ = viper.BindEnv("tdengine.port", "TAOS_KEEPER_TDENGINE_PORT")
	pflag.Int("tdengine.port", 6041, `TDengine REST server(taosAdapter)'s port. Env "TAOS_KEEPER_TDENGINE_PORT"`)

	viper.SetDefault("tdengine.username", "root")
	_ = viper.BindEnv("tdengine.username", "TAOS_KEEPER_TDENGINE_USERNAME")
	pflag.String("tdengine.username", "root", `TDengine server's username. Env "TAOS_KEEPER_TDENGINE_USERNAME"`)

	viper.SetDefault("tdengine.password", "taosdata")
	_ = viper.BindEnv("tdengine.password", "TAOS_KEEPER_TDENGINE_PASSWORD")
	pflag.String("tdengine.password", "taosdata", `TDengine server's password. Env "TAOS_KEEPER_TDENGINE_PASSWORD"`)

	viper.SetDefault("taosAdapter.address", "")
	_ = viper.BindEnv("taosAdapter.address", "TAOS_KEEPER_TAOSADAPTER_ADDRESS")
	pflag.String("taosAdapter.address", "", `list of taosAdapter that need to be monitored, multiple values split with white space. Env "TAOS_KEEPER_TAOSADAPTER_ADDRESS"`)

	viper.SetDefault("metrics.prefix", "")
	_ = viper.BindEnv("metrics.prefix", "TAOS_KEEPER_METRICS_PREFIX")
	pflag.String("metrics.prefix", "", `prefix in metrics names. Env "TAOS_KEEPER_METRICS_PREFIX"`)

	viper.SetDefault("metrics.database", "log")
	_ = viper.BindEnv("metrics.database", "TAOS_KEEPER_METRICS_DATABASE")
	pflag.String("metrics.database", "log", `database for storing metrics data. Env "TAOS_KEEPER_METRICS_DATABASE"`)

	viper.SetDefault("metrics.tables", "")
	_ = viper.BindEnv("metrics.tables", "TAOS_KEEPER_METRICS_TABLES")
	pflag.String("metrics.tables", "", `export some tables that are not super table, multiple values split with white space. Env "TAOS_KEEPER_METRICS_TABLES"`)

	viper.SetDefault("environment.incgroup", false)
	_ = viper.BindEnv("environment.incgroup", "TAOS_KEEPER_ENVIRONMENT_INCGROUP")
	pflag.Bool("environment.incgroup", false, `whether running in cgroup. Env "TAOS_KEEPER_ENVIRONMENT_INCGROUP"`)
}
