package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	TDLib       TDLibConfig   `yaml:"tdlib"`
	Proxies     []ProxyConfig `yaml:"proxies"`
	Metrics     MetricsConfig `yaml:"metrics"`
	Web         WebConfig     `yaml:"web"`
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

type WebConfig struct {
	Auth AuthConfig `yaml:"auth"`
}

type AuthConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

const (
	ProxyTypeMTProto = "mtproto"
	ProxyTypeSOCKS5  = "socks5"
	ProxyTypeHTTP    = "http"
)

type ProxyConfig struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	Secret   string `yaml:"secret"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	HTTPOnly bool   `yaml:"http_only"`
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
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, fmt.Errorf("env overrides: %w", err)
	}

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
	for i := range cfg.Proxies {
		if cfg.Proxies[i].Type == "" {
			cfg.Proxies[i].Type = ProxyTypeMTProto
		}
	}
}

func applyEnvOverrides(cfg *Config) error {
	if v := os.Getenv("TDMETER_API_ID"); v != "" {
		id, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid TDMETER_API_ID %q: %w", v, err)
		}
		cfg.TDLib.APIID = int32(id)
	}
	if v := os.Getenv("TDMETER_API_HASH"); v != "" {
		cfg.TDLib.APIHash = v
	}
	if v := os.Getenv("TDMETER_AUTH_USERNAME"); v != "" {
		cfg.Web.Auth.Username = v
	}
	if v := os.Getenv("TDMETER_AUTH_PASSWORD"); v != "" {
		cfg.Web.Auth.Password = v
	}
	return nil
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

	if cfg.CheckInterval <= 0 {
		return fmt.Errorf("check_interval must be positive, got %s", cfg.CheckInterval)
	}
	if cfg.TCPTimeout <= 0 {
		return fmt.Errorf("tcp_timeout must be positive, got %s", cfg.TCPTimeout)
	}
	if cfg.TDLibTimeout <= 0 {
		return fmt.Errorf("tdlib_timeout must be positive, got %s", cfg.TDLibTimeout)
	}
	if cfg.Concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1, got %d", cfg.Concurrency)
	}

	if (cfg.Web.Auth.Username == "") != (cfg.Web.Auth.Password == "") {
		return fmt.Errorf("web.auth: both username and password must be set, or neither")
	}

	for i, p := range cfg.Proxies {
		if p.Name == "" {
			return fmt.Errorf("proxy[%d]: name is required", i)
		}
		if p.Server == "" {
			return fmt.Errorf("proxy[%d]: server is required", i)
		}
		if p.Port < 1 || p.Port > 65535 {
			return fmt.Errorf("proxy[%d]: port must be between 1 and 65535, got %d", i, p.Port)
		}
		if err := validateProxyType(p); err != nil {
			return fmt.Errorf("proxy[%d]: %w", i, err)
		}
	}

	return nil
}

func validateProxyType(p ProxyConfig) error {
	switch p.Type {
	case ProxyTypeMTProto:
		if p.Username != "" || p.Password != "" || p.HTTPOnly {
			return fmt.Errorf("username, password and http_only are not supported for mtproto proxies")
		}
		if p.Secret == "" {
			return fmt.Errorf("secret is required")
		}
		return validateSecret(p.Secret)
	case ProxyTypeSOCKS5:
		if p.Secret != "" {
			return fmt.Errorf("secret is only supported for mtproto proxies")
		}
		if p.HTTPOnly {
			return fmt.Errorf("http_only is only supported for http proxies")
		}
		// TDLib rejects SOCKS5 credentials of 128+ bytes at connect time.
		if len(p.Username) >= 128 || len(p.Password) >= 128 {
			return fmt.Errorf("socks5 username and password must be shorter than 128 bytes")
		}
		return nil
	case ProxyTypeHTTP:
		if p.Secret != "" {
			return fmt.Errorf("secret is only supported for mtproto proxies")
		}
		return nil
	default:
		return fmt.Errorf("unknown type %q, want %s, %s or %s", p.Type, ProxyTypeMTProto, ProxyTypeSOCKS5, ProxyTypeHTTP)
	}
}

func validateSecret(secret string) error {
	b, err := hex.DecodeString(secret)
	if err != nil {
		return fmt.Errorf("secret is not valid hex: %w", err)
	}

	// MTProto secrets: 16 bytes (no prefix), or 17+ bytes (dd/ee prefix + 16-byte key).
	if len(b) < 16 {
		return fmt.Errorf("secret too short: got %d bytes, need at least 16", len(b))
	}

	return nil
}
