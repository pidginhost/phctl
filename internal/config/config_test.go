package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.APIURL != DefaultAPIURL {
		t.Errorf("default APIURL = %q, want %q", cfg.APIURL, DefaultAPIURL)
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

func TestDir(t *testing.T) {
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("Dir() = %q, want absolute path", dir)
	}
	if filepath.Base(dir) != "phctl" {
		t.Errorf("Dir() base = %q, want %q", filepath.Base(dir), "phctl")
	}
}

func TestPath(t *testing.T) {
	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("Path() base = %q, want %q", filepath.Base(path), "config.yaml")
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// Should return defaults
	if cfg.APIURL != DefaultAPIURL {
		t.Errorf("APIURL = %q, want default", cfg.APIURL)
	}
	if cfg.Output != "table" {
		t.Errorf("Output = %q, want %q", cfg.Output, "table")
	}
}

func TestLoadFromFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	configDir := filepath.Join(tmp, ".config", "phctl")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("auth_token: from-file\napi_url: https://custom.example.com\noutput: json\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.AuthToken != "from-file" {
		t.Errorf("AuthToken = %q, want %q", cfg.AuthToken, "from-file")
	}
	if cfg.APIURL != "https://custom.example.com" {
		t.Errorf("APIURL = %q, want %q", cfg.APIURL, "https://custom.example.com")
	}
	if cfg.Output != "json" {
		t.Errorf("Output = %q, want %q", cfg.Output, "json")
	}
}

func TestEnvOverridesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Write a config file
	configDir := filepath.Join(tmp, ".config", "phctl")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("auth_token: file-token\napi_url: https://file-url.com\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Set env vars that should override
	t.Setenv("PIDGINHOST_API_TOKEN", "env-token")
	t.Setenv("PIDGINHOST_API_URL", "https://env-url.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.AuthToken != "env-token" {
		t.Errorf("AuthToken = %q, want %q (env should override file)", cfg.AuthToken, "env-token")
	}
	if cfg.APIURL != "https://env-url.com" {
		t.Errorf("APIURL = %q, want %q (env should override file)", cfg.APIURL, "https://env-url.com")
	}
}

func TestSavePreservesExistingValues(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	// Save initial config with token
	if err := Save(&Config{AuthToken: "first-token"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Save again with a different token — should overwrite token
	if err := Save(&Config{AuthToken: "second-token"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.AuthToken != "second-token" {
		t.Errorf("AuthToken = %q, want %q", cfg.AuthToken, "second-token")
	}
}

func TestSaveAtomic_PreservesExistingOnFailure(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	if err := Save(&Config{AuthToken: "original"}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	path, _ := Path()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	old := writeAtomic
	writeAtomic = func(string, []byte, os.FileMode) error {
		return errors.New("simulated atomic write failure")
	}
	t.Cleanup(func() { writeAtomic = old })

	if err := Save(&Config{AuthToken: "would-overwrite"}); err == nil {
		t.Fatal("expected error when atomic write fails")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("config was modified despite save failure\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	configDir := filepath.Join(tmp, ".config", "phctl")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("{{invalid yaml"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
