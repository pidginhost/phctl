package dedicated

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDedicatedCommandStructure(t *testing.T) {
	if Cmd.Use != "dedicated" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "dedicated")
	}
	found := false
	for _, a := range Cmd.Aliases {
		if a == "ded" {
			found = true
		}
	}
	if !found {
		t.Errorf("Aliases = %v, want to contain 'ded'", Cmd.Aliases)
	}
}

func TestDedicatedSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}
	if !names["server"] {
		t.Error("missing subcommand 'server'")
	}
}

func TestServerSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range serverCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get", "power", "reinstall", "rdns"} {
		if !names[want] {
			t.Errorf("server missing subcommand %q", want)
		}
	}
}

func TestPowerFlags(t *testing.T) {
	f := serverPowerCmd.Flags().Lookup("action")
	if f == nil {
		t.Fatal("missing --action flag on power command")
	}
}

func TestReinstallFlags(t *testing.T) {
	f := serverReinstallCmd.Flags().Lookup("os-id")
	if f == nil {
		t.Fatal("missing --os-id flag on reinstall command")
	}
}

func TestRDNSFlags(t *testing.T) {
	for _, flag := range []string{"ip-id", "hostname"} {
		f := serverRDNSCmd.Flags().Lookup(flag)
		if f == nil {
			t.Fatalf("missing --%s flag on rdns command", flag)
		}
	}
}

// runDedicated drives a command's RunE against a stub API answering payload.
func runDedicated(t *testing.T, cmd *cobra.Command, args []string, format, payload string) (string, string) {
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
	if err := cmd.RunE(child, args); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	return out.String(), path
}

// The API sends ips as a list of objects and server_status as an object (or
// null). A model declaring both as strings fails to decode every server.
const dedicatedServerJSON = `{"id":7,"hostname":"metal-1","status":"active","price":"99.00",` +
	`"next_invoice":"2026-11-01","created":"2026-01-01T00:00:00Z","billing_cycle":"monthly",` +
	`"server_status":{"status":"on","statusText":"Powered on"},` +
	`"ips":[{"id":1,"ip":"192.0.2.10","reverse_dns":"metal-1.example.com"},` +
	`{"id":2,"ip":"2001:db8::10","reverse_dns":""}],"os_name":null}`

func TestServerListRendersAddressListAndStatus(t *testing.T) {
	out, path := runDedicated(t, serverListCmd, nil, "table",
		`{"count":1,"next":null,"previous":null,"results":[`+dedicatedServerJSON+`]}`)
	if path != "/api/dedicated/servers/" {
		t.Errorf("path = %q, want /api/dedicated/servers/", path)
	}
	for _, want := range []string{"metal-1", "Powered on", "192.0.2.10, 2001:db8::10", "<none>"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q missing %q", out, want)
		}
	}
}

func TestServerGetRendersAddressListAndStatus(t *testing.T) {
	out, path := runDedicated(t, serverGetCmd, []string{"7"}, "table", dedicatedServerJSON)
	if path != "/api/dedicated/servers/7/" {
		t.Errorf("path = %q, want /api/dedicated/servers/7/", path)
	}
	for _, want := range []string{"Server Status:  Powered on", "IPs:            192.0.2.10, 2001:db8::10", "OS:             <none>", "Price:          99.00"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q missing %q", out, want)
		}
	}
}

func TestServerGetJSONKeepsTheAddressObjects(t *testing.T) {
	out, _ := runDedicated(t, serverGetCmd, []string{"7"}, "json", dedicatedServerJSON)
	var got struct {
		Ips []struct {
			Ip         string `json:"ip"`
			ReverseDns string `json:"reverse_dns"`
		} `json:"ips"`
		ServerStatus struct {
			StatusText string `json:"statusText"`
		} `json:"server_status"`
		Price string `json:"price"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(got.Ips) != 2 || got.Ips[0].Ip != "192.0.2.10" || got.Ips[0].ReverseDns != "metal-1.example.com" {
		t.Errorf("ips = %+v, want both address objects", got.Ips)
	}
	if got.ServerStatus.StatusText != "Powered on" {
		t.Errorf("server_status = %+v, want statusText Powered on", got.ServerStatus)
	}
	if got.Price != "99.00" {
		t.Errorf("price = %q, want the string 99.00", got.Price)
	}
}

func TestServerListToleratesNullServerStatus(t *testing.T) {
	payload := strings.Replace(dedicatedServerJSON, `{"status":"on","statusText":"Powered on"}`, `null`, 1)
	out, _ := runDedicated(t, serverListCmd, nil, "table",
		`{"count":1,"next":null,"previous":null,"results":[`+payload+`]}`)
	if !strings.Contains(out, "metal-1") {
		t.Errorf("output %q missing the server", out)
	}
}

func TestServerGetRejectsNullResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`null`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	for name, cmd := range map[string]*cobra.Command{"get": serverGetCmd, "list": serverListCmd} {
		child := &cobra.Command{Use: "child"}
		child.SetOut(&bytes.Buffer{})
		if err := cmd.RunE(child, []string{"7"}); err == nil {
			t.Errorf("%s: RunE returned nil for a null body, want an error", name)
		}
	}
}
