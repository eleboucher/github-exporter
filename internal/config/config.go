package config

import (
	"bytes"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/caarlos0/env/v11"
	"gopkg.in/yaml.v3"
)

type AggregateType string

const (
	AggregateSum   AggregateType = "sum"
	AggregateCount AggregateType = "count"
	AggregateMax   AggregateType = "max"

	DefaultGitHubAPIURL = "https://api.github.com"
)

type MetricConfig struct {
	Name      string            `yaml:"name"`
	Path      string            `yaml:"path"`
	Help      string            `yaml:"help"`
	Aggregate AggregateType     `yaml:"aggregate"` // sum, count, max
	Labels    map[string]string `yaml:"labels"`
}

type RequestConfig struct {
	ApiPath string         `yaml:"api_path"`
	Method  string         `yaml:"method"`
	Body    string         `yaml:"body"`
	Metrics []MetricConfig `yaml:"metrics"`
}

type Config struct {
	GithubAPIURL   string          `env:"GITHUB_API_URL" yaml:"github_api_url" `
	Token          string          `env:"GITHUB_TOKEN" yaml:"github_token"`
	ScrapeInterval string          `yaml:"scrape_interval"`
	Requests       []RequestConfig `yaml:"requests"`
}

func getEnvMap(githubUser string) map[string]string {
	items := make(map[string]string)
	for _, item := range os.Environ() {
		splits := strings.SplitN(item, "=", 2)
		if len(splits) == 2 {
			items[splits[0]] = splits[1]
		}
	}
	if githubUser != "" {
		items["GITHUB_USER"] = githubUser
	}
	return items
}

func Load(path string, githubUser string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("config").Parse(string(data))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, getEnvMap(githubUser)); err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(buf.Bytes(), &cfg); err != nil {
		return nil, err
	}

	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	if cfg.ScrapeInterval == "" {
		cfg.ScrapeInterval = "15m"
	}

	if cfg.GithubAPIURL == "" {
		cfg.GithubAPIURL = DefaultGitHubAPIURL
	}
	cfg.GithubAPIURL = strings.TrimRight(cfg.GithubAPIURL, "/")
	return &cfg, nil
}

func (c *Config) GetDuration() time.Duration {
	d, _ := time.ParseDuration(c.ScrapeInterval)
	if d == 0 {
		return 15 * time.Minute
	}
	return d
}
