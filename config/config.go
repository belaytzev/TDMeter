package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	TDLib       TDLibConfig   `yaml:"tdlib"`
	Proxies     []ProxyConfig `yaml:"proxies"`
	Metrics     MetricsConfig `yaml:"metrics"`
	Concurrency int           `yaml:"concurrency"`

	CheckInterval time.Duration `yaml:"check_interval"`
	TCPTimeout    time.Duration `yaml:"tcp_timeout"`
	TDLibTimeout  time.Duration `yaml:"tdlib_timeout"`
}

type TDLibConfig struct {
	APIID   int32  `yaml:"api_id"`
	APIHash string `yaml:"api_hash"`
	DBPath  string `yaml:"db_path"`
}

type MetricsConfig struct {
	Listen string `yaml:"listen"`
}

type ProxyConfig struct {
	Name   string `yaml:"name"`
	Server string `yaml:"server"`
	Port   int    `yaml:"port"`
	Secret string `yaml:"secret"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	applyDefaults(cfg)
	applyEnvOverrides(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 60 * time.Second
	}
	if cfg.TCPTimeout == 0 {
		cfg.TCPTimeout = 5 * time.Second
	}
	if cfg.TDLibTimeout == 0 {
		cfg.TDLibTimeout = 10 * time.Second
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 5
	}
	if cfg.Metrics.Listen == "" {
		cfg.Metrics.Listen = ":2112"
	}
	if cfg.TDLib.DBPath == "" {
		cfg.TDLib.DBPath = "/tmp/tdmeter-tdlib/"
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("TDMETER_API_ID"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 32); err == nil {
			cfg.TDLib.APIID = int32(id)
		}
	}
	if v := os.Getenv("TDMETER_API_HASH"); v != "" {
		cfg.TDLib.APIHash = v
	}
}

func validate(cfg *Config) error {
	if cfg.TDLib.APIID == 0 {
		return fmt.Errorf("tdlib.api_id is required")
	}
	if cfg.TDLib.APIHash == "" {
		return fmt.Errorf("tdlib.api_hash is required")
	}
	if len(cfg.Proxies) == 0 {
		return fmt.Errorf("at least one proxy is required")
	}

	for i, p := range cfg.Proxies {
		if p.Server == "" {
			return fmt.Errorf("proxy[%d]: server is required", i)
		}
		if p.Port < 1 || p.Port > 65535 {
			return fmt.Errorf("proxy[%d]: port must be between 1 and 65535, got %d", i, p.Port)
		}
		if p.Secret == "" {
			return fmt.Errorf("proxy[%d]: secret is required", i)
		}
		if err := validateSecret(p.Secret); err != nil {
			return fmt.Errorf("proxy[%d]: %w", i, err)
		}
	}

	return nil
}

func validateSecret(secret string) error {
	// Strip known prefixes for hex validation
	s := secret
	if strings.HasPrefix(s, "ee") || strings.HasPrefix(s, "dd") {
		s = s[2:]
	}

	if len(s) == 0 {
		return fmt.Errorf("secret is empty after prefix")
	}

	if _, err := hex.DecodeString(s); err != nil {
		return fmt.Errorf("secret is not valid hex: %w", err)
	}

	return nil
}
