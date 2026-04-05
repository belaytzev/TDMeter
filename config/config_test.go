package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

const validConfig = `
tdlib:
  api_id: 12345
  api_hash: "abc123def456"
proxies:
  - name: proxy1
    server: example.com
    port: 443
    secret: "ee0123456789abcdef0123456789abcdef"
`

func TestLoad_ValidConfig(t *testing.T) {
	path := writeConfig(t, validConfig)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.TDLib.APIID != 12345 {
		t.Errorf("api_id = %d, want 12345", cfg.TDLib.APIID)
	}
	if cfg.TDLib.APIHash != "abc123def456" {
		t.Errorf("api_hash = %q, want %q", cfg.TDLib.APIHash, "abc123def456")
	}
	if len(cfg.Proxies) != 1 {
		t.Fatalf("proxies count = %d, want 1", len(cfg.Proxies))
	}
	if cfg.Proxies[0].Name != "proxy1" {
		t.Errorf("proxy name = %q, want %q", cfg.Proxies[0].Name, "proxy1")
	}
	if cfg.Proxies[0].Server != "example.com" {
		t.Errorf("proxy server = %q, want %q", cfg.Proxies[0].Server, "example.com")
	}
	if cfg.Proxies[0].Port != 443 {
		t.Errorf("proxy port = %d, want 443", cfg.Proxies[0].Port)
	}
}

func TestLoad_Defaults(t *testing.T) {
	path := writeConfig(t, validConfig)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.CheckInterval != 60*time.Second {
		t.Errorf("check_interval = %v, want 60s", cfg.CheckInterval)
	}
	if cfg.TCPTimeout != 5*time.Second {
		t.Errorf("tcp_timeout = %v, want 5s", cfg.TCPTimeout)
	}
	if cfg.TDLibTimeout != 10*time.Second {
		t.Errorf("tdlib_timeout = %v, want 10s", cfg.TDLibTimeout)
	}
	if cfg.Concurrency != 5 {
		t.Errorf("concurrency = %d, want 5", cfg.Concurrency)
	}
	if cfg.Metrics.Listen != ":2112" {
		t.Errorf("listen = %q, want %q", cfg.Metrics.Listen, ":2112")
	}
	if cfg.TDLib.DBPath != "/tmp/tdmeter-tdlib/" {
		t.Errorf("db_path = %q, want %q", cfg.TDLib.DBPath, "/tmp/tdmeter-tdlib/")
	}
}

func TestLoad_CustomValues(t *testing.T) {
	content := `
tdlib:
  api_id: 99999
  api_hash: "custom_hash"
  db_path: "/custom/path/"
proxies:
  - name: p1
    server: 1.2.3.4
    port: 8080
    secret: "dd0123456789abcdef0123456789abcdef"
metrics:
  listen: ":9090"
concurrency: 10
check_interval: 30s
tcp_timeout: 3s
tdlib_timeout: 15s
`
	path := writeConfig(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Concurrency != 10 {
		t.Errorf("concurrency = %d, want 10", cfg.Concurrency)
	}
	if cfg.CheckInterval != 30*time.Second {
		t.Errorf("check_interval = %v, want 30s", cfg.CheckInterval)
	}
	if cfg.Metrics.Listen != ":9090" {
		t.Errorf("listen = %q, want %q", cfg.Metrics.Listen, ":9090")
	}
	if cfg.TDLib.DBPath != "/custom/path/" {
		t.Errorf("db_path = %q, want %q", cfg.TDLib.DBPath, "/custom/path/")
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	content := `
tdlib:
  api_id: 11111
  api_hash: "original_hash"
proxies:
  - name: p1
    server: example.com
    port: 443
    secret: "ee0123456789abcdef0123456789abcdef"
`
	path := writeConfig(t, content)

	t.Setenv("TDMETER_API_ID", "99999")
	t.Setenv("TDMETER_API_HASH", "overridden_hash")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.TDLib.APIID != 99999 {
		t.Errorf("api_id = %d, want 99999 (from env)", cfg.TDLib.APIID)
	}
	if cfg.TDLib.APIHash != "overridden_hash" {
		t.Errorf("api_hash = %q, want %q (from env)", cfg.TDLib.APIHash, "overridden_hash")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeConfig(t, ":::invalid yaml")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidation_MissingAPIID(t *testing.T) {
	content := `
tdlib:
  api_hash: "abc123"
proxies:
  - name: p1
    server: example.com
    port: 443
    secret: "ee0123456789abcdef0123456789abcdef"
`
	path := writeConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing api_id")
	}
}

func TestValidation_MissingAPIHash(t *testing.T) {
	content := `
tdlib:
  api_id: 12345
proxies:
  - name: p1
    server: example.com
    port: 443
    secret: "ee0123456789abcdef0123456789abcdef"
`
	path := writeConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing api_hash")
	}
}

func TestValidation_NoProxies(t *testing.T) {
	content := `
tdlib:
  api_id: 12345
  api_hash: "abc123"
proxies: []
`
	path := writeConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty proxies")
	}
}

func TestValidation_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too_high", 70000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := `
tdlib:
  api_id: 12345
  api_hash: "abc123"
proxies:
  - name: p1
    server: example.com
    port: ` + itoa(tt.port) + `
    secret: "ee0123456789abcdef0123456789abcdef"
`
			path := writeConfig(t, content)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("expected error for port %d", tt.port)
			}
		})
	}
}

func TestValidation_EmptySecret(t *testing.T) {
	content := `
tdlib:
  api_id: 12345
  api_hash: "abc123"
proxies:
  - name: p1
    server: example.com
    port: 443
    secret: ""
`
	path := writeConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestValidation_InvalidSecretHex(t *testing.T) {
	content := `
tdlib:
  api_id: 12345
  api_hash: "abc123"
proxies:
  - name: p1
    server: example.com
    port: 443
    secret: "ZZZZ"
`
	path := writeConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid hex secret")
	}
}

func TestValidateSecret_ValidSecrets(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{"simple_hex", "0123456789abcdef"},
		{"ee_prefix", "ee0123456789abcdef0123456789abcdef"},
		{"dd_prefix", "dd0123456789abcdef0123456789abcdef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateSecret(tt.secret); err != nil {
				t.Errorf("validateSecret(%q) returned error: %v", tt.secret, err)
			}
		})
	}
}

func TestValidateSecret_InvalidSecrets(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{"only_ee_prefix", "ee"},
		{"only_dd_prefix", "dd"},
		{"invalid_hex", "GHIJ"},
		{"odd_length_hex", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateSecret(tt.secret); err == nil {
				t.Errorf("validateSecret(%q) expected error, got nil", tt.secret)
			}
		})
	}
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
