package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaskToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"abc", "***"},
		{"12345678", "********"},
		{"123456789", "1234...6789"},
		{"abcdefghijklmnop", "abcd...mnop"},
	}

	for _, tt := range tests {
		got := maskToken(tt.input)
		if got != tt.want {
			t.Errorf("maskToken(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAuthCommandStructure(t *testing.T) {
	if Cmd.Use != "auth" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "auth")
	}
}

func TestAuthSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"init", "set", "status"} {
		if !names[want] {
			t.Errorf("auth missing subcommand %q", want)
		}
	}
}

func TestSetCmd(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	err := setCmd.RunE(setCmd, []string{"my-test-token"})
	if err != nil {
		t.Fatalf("set RunE error: %v", err)
	}

	// Verify token was saved
	data, err := os.ReadFile(filepath.Join(tmp, ".config", "phctl", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !contains(string(data), "my-test-token") {
		t.Errorf("config file should contain token, got: %s", data)
	}
}

func TestStatusCmdNoToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	// Should not error, just print "Not authenticated"
	err := statusCmd.RunE(statusCmd, nil)
	if err != nil {
		t.Fatalf("status RunE error: %v", err)
	}
}

func TestStatusCmdWithToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "abcdefghijklmnop")
	t.Setenv("PIDGINHOST_API_URL", "")

	err := statusCmd.RunE(statusCmd, nil)
	if err != nil {
		t.Fatalf("status RunE error: %v", err)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
