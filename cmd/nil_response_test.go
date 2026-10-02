package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Exercise the real RunE handlers and SDK decoder. Both an empty HTTP body and
// JSON null decode to a nil response without an SDK error. Check JSON as well as
// table output: a missing resource must not silently become a successful null.
func TestCommandsRejectMissingResponses(t *testing.T) {
	commands := []string{
		"account ssh-key list",
		"account company list",
		"account email list",
		"billing funds log",
		"billing deposit create --amount 125",
		"billing invoice get 42",
		"billing invoice pay 42",
		"billing service cancel 42",
		"billing service toggle-auto-pay 42",
		"compute firewall get 42",
		"compute firewall create --name example",
		"compute firewall rule create 42 --direction in --action ACCEPT",
		"compute floating-ip list",
		"compute floating-ip list --ipv6",
		"compute floating-ip create",
		"compute floating-ip create --ipv6",
		"compute floating-ip authorizations 42",
		"compute floating-ip authorizations 42 --ipv6",
		"compute floating-ip reverse-dns 42",
		"compute floating-ip reverse-dns 42 --ipv6",
		"compute floating-ip reverse-dns 42 --hostname example.test",
		"compute floating-ip reverse-dns 42 --ipv6 --hostname example.test",
		"compute image list",
		"compute ipv4 list",
		"compute ipv4 create",
		"compute ipv4 detach 42",
		"compute ipv4 reverse-dns 42",
		"compute ipv4 reverse-dns 42 --hostname example.test",
		"compute ipv6 list",
		"compute ipv6 create",
		"compute ipv6 detach 42",
		"compute ipv6 reverse-dns 42",
		"compute ipv6 reverse-dns 42 --hostname example.test",
		"compute network list",
		"compute network get 42",
		"compute network create --address 10.0.0.0/24",
		"compute network add-server 42 --server example.test",
		"compute network remove-server 42 --server example.test",
		"compute package list",
		"compute server list",
		"compute server get 42",
		"compute server create --image debian --package small",
		"compute server console 42",
		"compute volume get 42",
		"compute volume detach 42",
		"dedicated server power 42 --action start",
		"dedicated server rdns 42 --ip-id 1 --hostname example.test",
		"domain create example.test",
		"domain check example.test",
		"domain transfer example.ro --auth-code not-a-real-code",
		"domain registrant list",
		"domain registrant get 42",
		"domain registrant create",
		"freedns domain activate example.test --ip 192.0.2.1",
		"freedns domain deactivate example.test",
		"freedns record create --source internal --domain example.test --type A --name www --address 192.0.2.1",
		"freedns record delete --source internal --domain example.test --line 1",
		"hosting change-password 42 --password not-a-real-password",
		"kubernetes cluster create --type dev --package small",
		"kubernetes cluster upgrade-kube 42",
		"kubernetes cluster upgrade-talos 42",
		"kubernetes cluster connect-vm 42 --server 1",
		"kubernetes cluster disconnect-vm 42 --server 1",
		"kubernetes cluster connected-vms 42",
		"kubernetes types",
		"kubernetes pool list 42",
		"kubernetes pool create 42 --package small --size 1",
		"kubernetes node list 42 1",
		"support ticket list",
		"support ticket get 42",
		"support ticket create --subject Example --department 1 --message Example",
	}
	for _, command := range commands {
		for _, format := range []string{"table", "json"} {
			for _, body := range []string{"", "null"} {
				t.Run(fmt.Sprintf("%s/%s/body=%q", command, format, body), func(t *testing.T) {
					out, err := runResponseCommand(t, command, format, body)
					if err == nil || !strings.Contains(err.Error(), "server returned no") {
						t.Errorf("error = %v, want a missing-response error", err)
					}
					if out != "" {
						t.Errorf("unexpected success output: %q", out)
					}
				})
			}
		}
	}
}

// Use a fresh command for flags and I/O, restoring package-level flag variables
// afterward. No root Execute hooks run and every HTTP request stays on loopback.
func runResponseCommand(t *testing.T, invocation, format, body string) (string, error) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "not-a-real-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	original, args, err := rootCmd.Find(strings.Fields(invocation))
	if err != nil || original.RunE == nil {
		t.Fatalf("finding %q: %v", invocation, err)
	}
	command := &cobra.Command{Use: original.Use}
	command.Flags().String("output", format, "")
	command.Flags().Bool("force", true, "")
	addFlags := func(flags *pflag.FlagSet) {
		flags.VisitAll(func(flag *pflag.Flag) {
			if command.Flags().Lookup(flag.Name) != nil {
				return
			}
			value, changed := flag.Value.String(), flag.Changed
			t.Cleanup(func() {
				if err := flag.Value.Set(value); err != nil {
					t.Errorf("restoring --%s: %v", flag.Name, err)
				}
				flag.Changed = changed
			})
			command.Flags().AddFlag(flag)
		})
	}
	addFlags(original.Flags())
	for parent := original.Parent(); parent != nil; parent = parent.Parent() {
		addFlags(parent.PersistentFlags())
	}
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetIn(strings.NewReader(""))
	command.SetContext(context.Background())
	if err := command.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	if err := original.ValidateArgs(command.Flags().Args()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if requests.Load() != 1 {
			t.Errorf("API requests = %d, want 1 (test must reach the SDK)", requests.Load())
		}
		if recovered := recover(); recovered != nil {
			t.Errorf("command panicked on response %q: %v", body, recovered)
		}
	}()
	err = original.RunE(command, command.Flags().Args())
	return out.String(), err
}

func TestCommandsCheckOperationResult(t *testing.T) {
	for _, tc := range []struct {
		command string
		field   string
	}{
		{"compute ipv4 detach 42", "detached"},
		{"compute ipv6 detach 42", "detached"},
		{"compute volume detach 42", "detached"},
		{"compute network add-server 42 --server example.test", "created"},
		{"compute network remove-server 42 --server example.test", "removed"},
	} {
		for _, format := range []string{"table", "json"} {
			for _, applied := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/%t", tc.command, format, applied), func(t *testing.T) {
					body := fmt.Sprintf(`{"%s":%t}`, tc.field, applied)
					out, err := runResponseCommand(t, tc.command, format, body)
					if applied {
						if err != nil || out == "" {
							t.Fatalf("successful operation: out=%q, err=%v", out, err)
						}
					} else if err == nil || out != "" {
						t.Fatalf("failed operation reported success: out=%q, err=%v", out, err)
					}
				})
			}
		}
	}
}
