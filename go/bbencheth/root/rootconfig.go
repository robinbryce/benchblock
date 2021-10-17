package root

import (
	"time"
)

const (
	ConfigName = "bbencheth"
)

type Config struct {
	EthEndpoint   string
	BasePort      int
	ClientTimeout time.Duration `mapstructure:"client_timeout"`
	ResolveHosts  bool
	Retries       int
	NoProgress    bool `mapstructure:"no_progress"`
}

func (cfg *Config) SetDefaults() {
	cfg.EthEndpoint = ""
	cfg.BasePort = 8300
	cfg.ClientTimeout = 60 * time.Second
	cfg.ResolveHosts = true
	cfg.Retries = 10
}

func NewConfig() Config {
	cfg := Config{}
	cfg.SetDefaults()
	return cfg
}
