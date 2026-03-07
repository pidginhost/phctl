package client

import (
	"os"
	"strings"
	"testing"
)

func TestNewNoToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	_, err := New()
	if err == nil {
		t.Fatal("expected error when no token configured")
	}
	if !strings.Contains(err.Error(), "no API token") {
		t.Errorf("error = %q, want it to mention 'no API token'", err)
	}
}

func TestNewWithToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token-123")
	t.Setenv("PIDGINHOST_API_URL", "")

	client, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewWithConfigFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	// Create config file with token
	configDir := tmp + "/.config/phctl"
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(configDir+"/config.yaml", []byte("auth_token: file-token\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	client, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}
