package root

import (
	"time"
)

const (
	ConfigName = "bbeth"
)

type Config struct {
	EthEndpoint   string
	BasePort      int
	ClientTimeout time.Duration `mapstructure:"client-timeout"`
	ResolveHosts  bool
	Retries       int
	NoProgress    bool `mapstructure:"no-progress"`
}

func (cfg *Config) SetDefaults() {
	cfg.EthEndpoint = ""
	cfg.BasePort = 8300
	cfg.ClientTimeout = 60 * time.Second
	cfg.ResolveHosts = true
	cfg.Retries = 50
}

func NewConfig() Config {
	cfg := Config{}
	cfg.SetDefaults()
	return cfg
}
