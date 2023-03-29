package config

type MetricsConfig struct {
	Prefix          string          `toml:"prefix"`
	Database        string          `toml:"database"`
	DatabaseOptions DatabaseOptions `toml:"databaseoptions"`
	Tables          map[string]struct{}
	Normals         []string `toml:"tables"`
}

type TaosAdapter struct {
	Addrs []string `toml:"address"`
}

type Metric struct {
	Alias  string            `toml:"alias"`
	Help   string            `toml:"help"`
	Unit   string            `toml:"unit"`
	Type   string            `toml:"type"`
	Labels map[string]string `toml:"labels"`
}

type Environment struct {
	InCGroup bool `toml:"whether running in cgroup"`
}

type DatabaseOptions struct {
	CacheModel string `toml:"cachemodel"`
}
