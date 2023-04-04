package config

type MetricsConfig struct {
	Prefix          string                 `toml:"prefix"`
	Database        string                 `toml:"database"`
	Tables          []string               `toml:"tables"`
	DatabaseOptions map[string]interface{} `toml:"databaseoptions"`
}

type TaosAdapter struct {
	Address []string `toml:"address"`
}

type Metric struct {
	Alias  string            `toml:"alias"`
	Help   string            `toml:"help"`
	Unit   string            `toml:"unit"`
	Type   string            `toml:"type"`
	Labels map[string]string `toml:"labels"`
}

type Environment struct {
	InCGroup bool `toml:"incgroup"`
}
