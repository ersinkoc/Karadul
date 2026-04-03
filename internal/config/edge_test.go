package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateServerConfig_TLSWithSelfSigned verifies TLS passes validation with self-signed enabled.
func TestValidateServerConfig_TLSWithSelfSigned(t *testing.T) {
	cfg := DefaultServerConfig()
	cfg.TLS.Enabled = true
	cfg.TLS.SelfSigned = true
	cfg.TLS.CertFile = ""
	cfg.TLS.KeyFile = ""
	if err := ValidateServerConfig(cfg); err != nil {
		t.Fatalf("TLS with self-signed should pass validation: %v", err)
	}
}

// TestValidateServerConfig_TLSWithCerts verifies TLS passes when cert and key files are set.
func TestValidateServerConfig_TLSWithCerts(t *testing.T) {
	cfg := DefaultServerConfig()
	cfg.TLS.Enabled = true
	cfg.TLS.SelfSigned = false
	cfg.TLS.CertFile = "/path/to/cert.pem"
	cfg.TLS.KeyFile = "/path/to/key.pem"
	if err := ValidateServerConfig(cfg); err != nil {
		t.Fatalf("TLS with cert files should pass validation: %v", err)
	}
}

// TestValidateServerConfig_TLSMissingKeyFile verifies TLS fails with cert but no key.
func TestValidateServerConfig_TLSMissingKeyFile(t *testing.T) {
	cfg := DefaultServerConfig()
	cfg.TLS.Enabled = true
	cfg.TLS.SelfSigned = false
	cfg.TLS.CertFile = "/path/to/cert.pem"
	cfg.TLS.KeyFile = ""
	if err := ValidateServerConfig(cfg); err == nil {
		t.Fatal("TLS with cert but no key should fail")
	}
}

// TestValidateServerConfig_TLSMissingCertFile verifies TLS fails with key but no cert.
func TestValidateServerConfig_TLSMissingCertFile(t *testing.T) {
	cfg := DefaultServerConfig()
	cfg.TLS.Enabled = true
	cfg.TLS.SelfSigned = false
	cfg.TLS.CertFile = ""
	cfg.TLS.KeyFile = "/path/to/key.pem"
	if err := ValidateServerConfig(cfg); err == nil {
		t.Fatal("TLS with key but no cert should fail")
	}
}

// TestValidateLogLevel_AllValidLevels verifies all accepted log levels.
func TestValidateLogLevel_AllValidLevels(t *testing.T) {
	levels := []string{"", "debug", "info", "warn", "error"}
	for _, level := range levels {
		if err := validateLogLevel(level); err != nil {
			t.Errorf("validateLogLevel(%q) should succeed, got: %v", level, err)
		}
	}
}

// TestValidateLogLevel_InvalidLevels verifies rejected log levels.
func TestValidateLogLevel_InvalidLevels(t *testing.T) {
	levels := []string{"trace", "verbose", "DEBUG", "Info", "WARNING", "fatal", "panic"}
	for _, level := range levels {
		if err := validateLogLevel(level); err == nil {
			t.Errorf("validateLogLevel(%q) should fail", level)
		}
	}
}

// TestValidateNodeConfig_EmptyRoutes verifies empty AdvertiseRoutes passes.
func TestValidateNodeConfig_EmptyRoutes(t *testing.T) {
	cfg := DefaultNodeConfig()
	cfg.ServerURL = "https://coord.example.com"
	cfg.AdvertiseRoutes = []string{}
	if err := ValidateNodeConfig(cfg); err != nil {
		t.Fatalf("empty routes should be valid: %v", err)
	}
}

// TestValidateNodeConfig_NilRoutes verifies nil AdvertiseRoutes passes.
func TestValidateNodeConfig_NilRoutes(t *testing.T) {
	cfg := DefaultNodeConfig()
	cfg.ServerURL = "https://coord.example.com"
	cfg.AdvertiseRoutes = nil
	if err := ValidateNodeConfig(cfg); err != nil {
		t.Fatalf("nil routes should be valid: %v", err)
	}
}

// TestValidateNodeConfig_MultipleValidRoutes verifies multiple valid CIDRs.
func TestValidateNodeConfig_MultipleValidRoutes(t *testing.T) {
	cfg := DefaultNodeConfig()
	cfg.ServerURL = "https://coord.example.com"
	cfg.AdvertiseRoutes = []string{"192.168.0.0/24", "10.0.0.0/8", "172.16.0.0/12"}
	if err := ValidateNodeConfig(cfg); err != nil {
		t.Fatalf("multiple valid routes should pass: %v", err)
	}
}

// TestValidateNodeConfig_EmptyDNSUpstream verifies empty DNS upstream passes.
func TestValidateNodeConfig_EmptyDNSUpstream(t *testing.T) {
	cfg := DefaultNodeConfig()
	cfg.ServerURL = "https://coord.example.com"
	cfg.DNS.Upstream = ""
	if err := ValidateNodeConfig(cfg); err != nil {
		t.Fatalf("empty DNS upstream should pass: %v", err)
	}
}

// TestValidateNodeConfig_HTTPServerURL verifies http:// prefix is accepted.
func TestValidateNodeConfig_HTTPServerURL(t *testing.T) {
	cfg := DefaultNodeConfig()
	cfg.ServerURL = "http://coord.example.com"
	if err := ValidateNodeConfig(cfg); err != nil {
		t.Fatalf("http:// server URL should be valid: %v", err)
	}
}

// TestValidateNodeConfig_ZeroListenPort verifies port 0 is accepted.
func TestValidateNodeConfig_ZeroListenPort(t *testing.T) {
	cfg := DefaultNodeConfig()
	cfg.ServerURL = "https://coord.example.com"
	cfg.ListenPort = 0
	if err := ValidateNodeConfig(cfg); err != nil {
		t.Fatalf("port 0 should be valid: %v", err)
	}
}

// TestValidateNodeConfig_MaxListenPort verifies port 65535 is accepted.
func TestValidateNodeConfig_MaxListenPort(t *testing.T) {
	cfg := DefaultNodeConfig()
	cfg.ServerURL = "https://coord.example.com"
	cfg.ListenPort = 65535
	if err := ValidateNodeConfig(cfg); err != nil {
		t.Fatalf("port 65535 should be valid: %v", err)
	}
}

// TestDefaultNodeConfig_Defaults verifies all expected defaults.
func TestDefaultNodeConfig_Defaults(t *testing.T) {
	cfg := DefaultNodeConfig()
	if cfg.LogLevel != "info" {
		t.Errorf("default log level: want info, got %q", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("default log format: want text, got %q", cfg.LogFormat)
	}
	if !cfg.DNS.Enabled {
		t.Error("default DNS should be enabled")
	}
	if cfg.DNS.Upstream != "1.1.1.1:53" {
		t.Errorf("default DNS upstream: want 1.1.1.1:53, got %q", cfg.DNS.Upstream)
	}
}

// TestDefaultServerConfig_Defaults verifies all expected server defaults.
func TestDefaultServerConfig_Defaults(t *testing.T) {
	cfg := DefaultServerConfig()
	if cfg.Addr != ":8080" {
		t.Errorf("default addr: want :8080, got %q", cfg.Addr)
	}
	if cfg.ApprovalMode != "auto" {
		t.Errorf("default approval mode: want auto, got %q", cfg.ApprovalMode)
	}
	if cfg.Subnet != "100.64.0.0/10" {
		t.Errorf("default subnet: want 100.64.0.0/10, got %q", cfg.Subnet)
	}
	if cfg.RateLimit != 100 {
		t.Errorf("default rate limit: want 100, got %d", cfg.RateLimit)
	}
	if !cfg.DERP.Enabled {
		t.Error("default DERP should be enabled")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("default log level: want info, got %q", cfg.LogLevel)
	}
}

// TestLoadNodeConfig_FlatWithAllFields verifies flat format with all fields populated.
func TestLoadNodeConfig_FlatWithAllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "full.json")
	content := `{
		"server_url": "https://coord.example.com",
		"hostname": "myhost",
		"auth_key": "auth123",
		"advertise_routes": ["10.0.0.0/8"],
		"accept_routes": true,
		"exit_node": "abc123",
		"advertise_exit_node": true,
		"dns": {"enabled": false, "override_system": true, "upstream": "8.8.8.8:53"},
		"log_level": "debug",
		"log_format": "json",
		"listen_port": 51820,
		"data_dir": "/tmp/karadul-test"
	}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadNodeConfig(path)
	if err != nil {
		t.Fatalf("LoadNodeConfig: %v", err)
	}
	if cfg.ServerURL != "https://coord.example.com" {
		t.Errorf("ServerURL: %q", cfg.ServerURL)
	}
	if cfg.Hostname != "myhost" {
		t.Errorf("Hostname: %q", cfg.Hostname)
	}
	if cfg.AuthKey != "auth123" {
		t.Errorf("AuthKey: %q", cfg.AuthKey)
	}
	if !cfg.AcceptRoutes {
		t.Error("AcceptRoutes should be true")
	}
	if cfg.ExitNode != "abc123" {
		t.Errorf("ExitNode: %q", cfg.ExitNode)
	}
	if !cfg.AdvertiseExitNode {
		t.Error("AdvertiseExitNode should be true")
	}
	if cfg.DNS.Enabled {
		t.Error("DNS.Enabled should be false")
	}
	if !cfg.DNS.OverrideSystem {
		t.Error("DNS.OverrideSystem should be true")
	}
	if cfg.DNS.Upstream != "8.8.8.8:53" {
		t.Errorf("DNS.Upstream: %q", cfg.DNS.Upstream)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel: %q", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat: %q", cfg.LogFormat)
	}
	if cfg.ListenPort != 51820 {
		t.Errorf("ListenPort: %d", cfg.ListenPort)
	}
	if cfg.DataDir != "/tmp/karadul-test" {
		t.Errorf("DataDir: %q", cfg.DataDir)
	}
}

// TestLoadServerConfig_FlatWithAllFields verifies flat format with all server fields populated.
func TestLoadServerConfig_FlatWithAllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "full.json")
	content := `{
		"addr": ":9999",
		"tls": {"enabled": true, "self_signed": true, "cert_file": "", "key_file": ""},
		"approval_mode": "manual",
		"subnet": "10.0.0.0/8",
		"data_dir": "/tmp/kserver",
		"derp": {"enabled": false, "addr": ":3478"},
		"log_level": "warn",
		"log_format": "json",
		"rate_limit": 50,
		"allowed_origins": ["https://app.example.com"],
		"admin_secret": "supersecret"
	}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadServerConfig(path)
	if err != nil {
		t.Fatalf("LoadServerConfig: %v", err)
	}
	if cfg.Addr != ":9999" {
		t.Errorf("Addr: %q", cfg.Addr)
	}
	if !cfg.TLS.Enabled || !cfg.TLS.SelfSigned {
		t.Error("TLS should be enabled with self-signed")
	}
	if cfg.ApprovalMode != "manual" {
		t.Errorf("ApprovalMode: %q", cfg.ApprovalMode)
	}
	if cfg.Subnet != "10.0.0.0/8" {
		t.Errorf("Subnet: %q", cfg.Subnet)
	}
	if cfg.DataDir != "/tmp/kserver" {
		t.Errorf("DataDir: %q", cfg.DataDir)
	}
	if cfg.DERP.Enabled {
		t.Error("DERP should be disabled")
	}
	if cfg.DERP.Addr != ":3478" {
		t.Errorf("DERP.Addr: %q", cfg.DERP.Addr)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel: %q", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat: %q", cfg.LogFormat)
	}
	if cfg.RateLimit != 50 {
		t.Errorf("RateLimit: %d", cfg.RateLimit)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "https://app.example.com" {
		t.Errorf("AllowedOrigins: %v", cfg.AllowedOrigins)
	}
	if cfg.AdminSecret != "supersecret" {
		t.Errorf("AdminSecret: %q", cfg.AdminSecret)
	}
}

// TestSaveNodeConfig_FileContent verifies saved JSON is valid and readable.
func TestSaveNodeConfig_FileContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := DefaultNodeConfig()
	cfg.ServerURL = "https://coord.example.com"
	cfg.Hostname = "test-host"

	if err := SaveNodeConfig(cfg, path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Should be valid JSON with indentation.
	if len(data) == 0 {
		t.Fatal("saved config is empty")
	}
	// Verify it can be loaded back.
	loaded, err := LoadNodeConfig(path)
	if err != nil {
		t.Fatalf("reload saved config: %v", err)
	}
	if loaded.Hostname != "test-host" {
		t.Errorf("hostname after reload: %q", loaded.Hostname)
	}
}

// TestSaveServerConfig_FileContent verifies saved server config is valid.
func TestSaveServerConfig_FileContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")
	cfg := DefaultServerConfig()
	cfg.Addr = ":7777"
	cfg.AdminSecret = "my-secret"

	if err := SaveServerConfig(cfg, path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("saved server config is empty")
	}
	loaded, err := LoadServerConfig(path)
	if err != nil {
		t.Fatalf("reload saved config: %v", err)
	}
	if loaded.Addr != ":7777" {
		t.Errorf("addr after reload: %q", loaded.Addr)
	}
	if loaded.AdminSecret != "my-secret" {
		t.Errorf("admin secret after reload: %q", loaded.AdminSecret)
	}
}

// TestLoadNodeConfig_NestedWithEmptyNode verifies nested format with empty node object falls back.
func TestLoadNodeConfig_NestedWithEmptyNode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty_node.json")
	content := `{"server": {"addr": ":8080"}, "node": {}}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	// Empty node object should trigger json.Unmarshal on empty RawMessage,
	// which succeeds with all defaults.
	cfg, err := LoadNodeConfig(path)
	if err != nil {
		t.Fatalf("LoadNodeConfig with empty node: %v", err)
	}
	// Should have defaults since node object is empty.
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log level, got %q", cfg.LogLevel)
	}
}

// TestLoadServerConfig_NestedWithNoServer verifies nested format without server key falls back to flat.
func TestLoadServerConfig_NestedWithNoServer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "flat.json")
	content := `{"addr": ":9999", "subnet": "10.0.0.0/8"}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadServerConfig(path)
	if err != nil {
		t.Fatalf("LoadServerConfig flat: %v", err)
	}
	if cfg.Addr != ":9999" {
		t.Errorf("Addr: %q", cfg.Addr)
	}
	if cfg.Subnet != "10.0.0.0/8" {
		t.Errorf("Subnet: %q", cfg.Subnet)
	}
}
