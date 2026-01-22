package config

import (
	"os"
	"time"

	"github.com/caarlos0/env/v10"
	"gopkg.in/yaml.v3"
)

type AggregateType string

const (
	AggregateSum   AggregateType = "sum"
	AggregateCount AggregateType = "count"
	AggregateMax   AggregateType = "max"
)

type MetricConfig struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`

	Help      string        `yaml:"help"`
	Aggregate AggregateType `yaml:"aggregate"` // sum, count, max
}

type RequestConfig struct {
	ApiPath string         `yaml:"api_path"`
	Method  string         `yaml:"method"`
	Body    string         `yaml:"body"`
	Metrics []MetricConfig `yaml:"metrics"`
}

type Config struct {
	Token          string          `env:"GITHUB_TOKEN" yaml:"github_token"`
	ScrapeInterval string          `yaml:"scrape_interval"`
	Requests       []RequestConfig `yaml:"requests"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	if cfg.ScrapeInterval == "" {
		cfg.ScrapeInterval = "15m"
	}

	return &cfg, nil
}

func (c *Config) GetDuration() time.Duration {
	d, _ := time.ParseDuration(c.ScrapeInterval)
	if d == 0 {
		return 15 * time.Minute
	}
	return d
}
