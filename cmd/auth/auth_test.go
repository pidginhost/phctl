package auth

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
	for _, want := range []string{"init", "set", "status", "login"} {
		if !names[want] {
			t.Errorf("auth missing subcommand %q", want)
		}
	}
}

func TestSetCmdReadsFromStdin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	setCmd.SetIn(strings.NewReader("my-test-token\n"))
	t.Cleanup(func() { setCmd.SetIn(nil) })

	if err := setCmd.RunE(setCmd, nil); err != nil {
		t.Fatalf("set RunE error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".config", "phctl", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !contains(string(data), "my-test-token") {
		t.Errorf("config file should contain token, got: %s", data)
	}
}

func TestSetCmdRejectsPositionalArg(t *testing.T) {
	if err := setCmd.Args(setCmd, []string{"my-token"}); err == nil {
		t.Error("set must not accept positional token args (would leak via ps and /proc/<pid>/cmdline)")
	}
}

func TestSetCmdRejectsEmptyStdin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	setCmd.SetIn(strings.NewReader("   \n"))
	t.Cleanup(func() { setCmd.SetIn(nil) })

	if err := setCmd.RunE(setCmd, nil); err == nil {
		t.Error("set should error on empty/whitespace stdin")
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

// TestStatusCmdHonoursOutputFlag pins the -o contract on a query command, and
// guards that the raw token never reaches machine-readable output — only the
// masked form the table view already shows.
func TestStatusCmdHonoursOutputFlag(t *testing.T) {
	const token = "abcdefghijklmnop"

	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", token)
	t.Setenv("PIDGINHOST_API_URL", "https://api.example.com")

	root := &cobra.Command{Use: "phctl"}
	root.PersistentFlags().StringP("output", "o", "json", "Output format")
	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)

	var out bytes.Buffer
	child.SetOut(&out)

	if err := statusCmd.RunE(child, nil); err != nil {
		t.Fatalf("status RunE error: %v", err)
	}

	var decoded struct {
		Authenticated bool   `json:"authenticated"`
		Token         string `json:"token"`
		APIURL        string `json:"api_url"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("-o json output is not valid JSON (%v): %q", err, out.String())
	}
	if !decoded.Authenticated {
		t.Error("authenticated = false, want true")
	}
	if decoded.APIURL != "https://api.example.com" {
		t.Errorf("api_url = %q, want %q", decoded.APIURL, "https://api.example.com")
	}
	if strings.Contains(out.String(), token) {
		t.Error("raw auth token leaked into -o json output")
	}
	if decoded.Token != maskToken(token) {
		t.Errorf("token = %q, want masked %q", decoded.Token, maskToken(token))
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
