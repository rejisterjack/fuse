package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is runtime configuration for the Fuse proxy.
type Config struct {
	ListenAddr       string  `yaml:"listen_addr"`
	DatabaseURL      string  `yaml:"database_url"`
	UpstreamBaseURL  string  `yaml:"upstream_base_url"`
	UpstreamAPIKey   string  `yaml:"upstream_api_key"`
	UpstreamProvider string  `yaml:"upstream_provider"`
	DefaultSoftUSD   float64 `yaml:"default_soft_usd"`
	DefaultHardUSD   float64 `yaml:"default_hard_usd"`
	SlackWebhookURL  string  `yaml:"slack_webhook_url"`
	SMTPFrom         string  `yaml:"smtp_from"`
	SMTPAddr         string  `yaml:"smtp_addr"`
	SMTPTo           string  `yaml:"smtp_to"`
}

// Load reads YAML from path and applies environment overrides.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if v := os.Getenv("FUSE_DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("FUSE_UPSTREAM_API_KEY"); v != "" {
		cfg.UpstreamAPIKey = v
	}
	if v := os.Getenv("FUSE_UPSTREAM_BASE_URL"); v != "" {
		cfg.UpstreamBaseURL = v
	}
	if v := os.Getenv("FUSE_SLACK_WEBHOOK_URL"); v != "" {
		cfg.SlackWebhookURL = v
	}
	if v := os.Getenv("FUSE_UPSTREAM_PROVIDER"); v != "" {
		cfg.UpstreamProvider = v
	}
	if v := os.Getenv("FUSE_SMTP_TO"); v != "" {
		cfg.SMTPTo = v
	}
	if v := os.Getenv("FUSE_SMTP_ADDR"); v != "" {
		cfg.SMTPAddr = v
	}
	if v := os.Getenv("FUSE_SMTP_FROM"); v != "" {
		cfg.SMTPFrom = v
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.UpstreamBaseURL == "" {
		cfg.UpstreamBaseURL = "https://api.openai.com"
	}
	if cfg.UpstreamProvider == "" {
		cfg.UpstreamProvider = "openai"
	}
	if cfg.DefaultHardUSD == 0 {
		cfg.DefaultHardUSD = 25
	}
	if cfg.DefaultSoftUSD == 0 {
		cfg.DefaultSoftUSD = 5
	}
	return cfg, nil
}
