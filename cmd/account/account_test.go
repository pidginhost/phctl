package account

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/pidginhost/phctl/internal/testutil/httptest"
	"github.com/spf13/cobra"
)

func newAccountTestCommand(t *testing.T, format string) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	root := &cobra.Command{Use: "phctl"}
	root.PersistentFlags().StringP("output", "o", format, "Output format")
	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)
	var out bytes.Buffer
	child.SetOut(&out)
	return child, &out
}

func TestAccountCommandStructure(t *testing.T) {
	if Cmd.Use != "account" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "account")
	}
}

func TestAccountSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"profile", "ssh-key", "company", "api-token", "email"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestSSHKeySubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range sshKeyCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "create", "delete"} {
		if !names[want] {
			t.Errorf("ssh-key missing subcommand %q", want)
		}
	}
}

func TestSSHKeyAliases(t *testing.T) {
	aliases := sshKeyCmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "ssh" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ssh-key Aliases = %v, want to contain 'ssh'", aliases)
	}
}

func TestSSHKeyDeleteAliases(t *testing.T) {
	aliases := sshKeyDeleteCmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "rm" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ssh-key delete Aliases = %v, want to contain 'rm'", aliases)
	}
}

func TestCompanySubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range companyCmd.Commands() {
		names[c.Name()] = true
	}

	if !names["list"] {
		t.Error("company missing subcommand 'list'")
	}
}

func TestSSHKeyCreateFlags(t *testing.T) {
	keyFlag := sshKeyCreateCmd.Flags().Lookup("key")
	if keyFlag == nil {
		t.Fatal("missing --key flag on ssh-key create")
	}

	aliasFlag := sshKeyCreateCmd.Flags().Lookup("alias")
	if aliasFlag == nil {
		t.Fatal("missing --alias flag on ssh-key create")
	}
}

func TestAPITokenCreateHandlesDarkModeResponse(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/account/api-tokens/"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":7,"name":"deploy","scope":"read_write","key":"secret","created":"2026-08-18T00:00:00Z"}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	previousName := apiTokenCreateName
	apiTokenCreateName = "deploy"
	t.Cleanup(func() { apiTokenCreateName = previousName })
	cmd, out := newAccountTestCommand(t, "table")
	if err := apiTokenCreateCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	for _, field := range []string{"account", "membership_status"} {
		value, present := body[field]
		if !present {
			t.Errorf("request omitted server-assigned field %q", field)
		} else if value != nil {
			t.Errorf("request field %q = %#v, want JSON null", field, value)
		}
	}
	if got, want := out.String(), "API token created (Name: deploy)\nToken: secret\nSave this token — it will not be shown again.\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestAPITokenListHandlesDarkModeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":1,"next":null,"previous":null,"results":[{"id":7,"name":"deploy","scope":"read_write","key_prefix":"abcd","created":"2026-08-18T00:00:00Z","last_used":null,"request_count":0}]}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	cmd, out := newAccountTestCommand(t, "table")
	if err := apiTokenListCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	for _, want := range []string{"ID", "NAME", "CREATED", "deploy", "2026-08-18T00:00:00Z"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output %q missing %q", out.String(), want)
		}
	}
}
