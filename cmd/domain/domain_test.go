package domain

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDomainCommandStructure(t *testing.T) {
	if Cmd.Use != "domain" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "domain")
	}

	aliases := Cmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "dns" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("domain Aliases = %v, want to contain 'dns'", aliases)
	}
}

func TestDomainSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "get", "create", "check", "renew", "cancel", "transfer", "nameservers", "tld", "registrant"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestDomainCreateFlags(t *testing.T) {
	ns := domainCreateCmd.Flags().Lookup("nameservers")
	if ns == nil {
		t.Fatal("missing --nameservers flag")
	}
	years := domainCreateCmd.Flags().Lookup("years")
	if years == nil {
		t.Fatal("missing --years flag")
	}
}

func TestDomainRenewFlags(t *testing.T) {
	years := domainRenewCmd.Flags().Lookup("years")
	if years == nil {
		t.Fatal("missing --years flag on renew")
		return
	}
	if years.DefValue != "1" {
		t.Errorf("renew --years default = %q, want %q", years.DefValue, "1")
	}
}

func TestTLDSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range tldCmd.Commands() {
		names[c.Name()] = true
	}
	if !names["list"] {
		t.Error("tld missing subcommand 'list'")
	}
}

func TestRegistrantSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range registrantCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "get", "create", "delete"} {
		if !names[want] {
			t.Errorf("registrant missing subcommand %q", want)
		}
	}
}

func TestRegistrantCreateFlags(t *testing.T) {
	required := []string{"first-name", "last-name", "email", "phone", "address", "city", "region", "postal-code", "country"}
	for _, name := range required {
		f := registrantCreateCmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("registrant create missing flag --%s", name)
		}
	}
}

func TestRegistrantDeleteAliases(t *testing.T) {
	aliases := registrantDeleteCmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "rm" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("registrant delete Aliases = %v, want to contain 'rm'", aliases)
	}
}

// runDomain drives a command's RunE against a stub API answering payload.
func runDomain(t *testing.T, cmd *cobra.Command, args []string, format, payload string) (string, string, error) {
	t.Helper()
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(server.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	root := &cobra.Command{Use: "phctl"}
	root.PersistentFlags().StringP("output", "o", format, "Output format")
	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)
	var out bytes.Buffer
	child.SetOut(&out)
	err := cmd.RunE(child, args)
	return out.String(), path, err
}

// The API answers a check with available and supported only.
func TestDomainCheckReportsAnswer(t *testing.T) {
	for _, tc := range []struct{ payload, want string }{
		{`{"available":true,"supported":true}`, "example.ro is available"},
		{`{"available":false,"supported":true}`, "example.ro is not available"},
		{`{"available":false,"supported":false}`, "example.ro is not supported"},
		{`{"available":true,"supported":false}`, "example.ro is not supported"},
	} {
		out, path, err := runDomain(t, domainCheckCmd, []string{"example.ro"}, "table", tc.payload)
		if err != nil {
			t.Fatalf("%s: RunE: %v", tc.payload, err)
		}
		if path != "/api/domain/domain/check-availability/" {
			t.Errorf("path = %q", path)
		}
		if !strings.Contains(out, tc.want) {
			t.Errorf("%s: output = %q, want %q", tc.payload, out, tc.want)
		}
	}
}

func TestDomainCheckJSONKeepsTheAnswer(t *testing.T) {
	for _, payload := range []string{
		`{"available":true,"supported":true}`,
		`{"available":false,"supported":true}`,
		`{"available":false,"supported":false}`,
		`{"available":true,"supported":false}`,
	} {
		t.Run(payload, func(t *testing.T) {
			out, _, err := runDomain(t, domainCheckCmd, []string{"example.ro"}, "json", payload)
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			var got, want map[string]bool
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("output is not JSON: %v: %s", err, out)
			}
			if err := json.Unmarshal([]byte(payload), &want); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("JSON = %v, want %v", got, want)
			}
			for key, expected := range want {
				if value, ok := got[key]; !ok || value != expected {
					t.Errorf("JSON = %v, want %v", got, want)
				}
			}
		})
	}
}
