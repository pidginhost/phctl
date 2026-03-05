package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.APIURL != "https://www.pidginhost.com" {
		t.Errorf("default APIURL = %q, want %q", cfg.APIURL, "https://www.pidginhost.com")
	}
	if cfg.Output != "table" {
		t.Errorf("default Output = %q, want %q", cfg.Output, "table")
	}
	if cfg.AuthToken != "" {
		t.Errorf("default AuthToken = %q, want empty", cfg.AuthToken)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", "https://custom.api.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.AuthToken != "test-token" {
		t.Errorf("AuthToken = %q, want %q", cfg.AuthToken, "test-token")
	}
	if cfg.APIURL != "https://custom.api.com" {
		t.Errorf("APIURL = %q, want %q", cfg.APIURL, "https://custom.api.com")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	// Ensure no env vars interfere
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	cfg := &Config{AuthToken: "saved-token"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists
	path := filepath.Join(tmp, ".config", "phctl", "config.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Load it back
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.AuthToken != "saved-token" {
		t.Errorf("loaded AuthToken = %q, want %q", loaded.AuthToken, "saved-token")
	}
}

func TestSaveDoesNotPersistEnvValues(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	// Set env vars
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "https://env-url.com")

	// Save only a token — should NOT persist env APIURL
	cfg := &Config{AuthToken: "my-token"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Read the raw file — should not contain the env URL
	path := filepath.Join(tmp, ".config", "phctl", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	content := string(data)
	if contains(content, "env-url.com") {
		t.Errorf("config file should not contain env-sourced URL, got:\n%s", content)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
