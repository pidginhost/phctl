package compute

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	gatewayJSON = `{"id":5,"name":"edge","state":"active","state_reason":"","status":"active",` +
		`"desired_revision":7,"applied_revision":7,"ipv4":"192.0.2.10","ipv6":null,` +
		`"maintenance_weekday":2,"maintenance_hour":3,"ha_enabled":false,"current_operation":null,` +
		`"created":"2026-11-01T10:00:00Z"}`

	gatewayOperationJSON = `{"id":"op-1","kind":"config_delete","status":"queued","network":null,` +
		`"revision":8,"started_at":"2026-11-14T18:00:00Z","deadline_at":null,"completed_at":null,` +
		`"servers":[],"outcomes":{},"reason":""}`

	// What the API answers a risky change with until the preview is sent back.
	gatewayDeletePreviewJSON = `{"message":"Confirm the change.","extra":{"fields":{},"preview":{"revision":7,` +
		`"servers":[{"id":42,"hostname":"web1"}],"networks":[{"id":3,"name":"net-a"}]}}}`
	gatewayPreviewJSON = `{"message":"Confirm the change.","extra":{"fields":{},"preview":{"revision":7,` +
		`"servers":[{"id":42,"hostname":"web1"}]}}}`
	gatewayBusyJSON = `{"message":"Another operation is running.","extra":{"fields":{}}}`

	gatewayNetworkJSON = `{"network":3,"role":"router","snat":true,"state":"attached","slot":1,` +
		`"address":"10.0.0.1","changed_revision":7,"pending":false,"reason":""}`
)

// setFlags sets flags the way the command line does, so Changed() is true, and
// restores both value and Changed so one test cannot leak into the next.
func setFlags(t *testing.T, cmd *cobra.Command, flags map[string]string) {
	t.Helper()
	for name, value := range flags {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("%s has no --%s flag", cmd.CommandPath(), name)
		}
		prev, changed := f.Value.String(), f.Changed
		restore := func() error { return f.Value.Set(prev) }
		// A slice flag's Set appends, and cannot parse its own String().
		if slice, ok := f.Value.(pflag.SliceValue); ok {
			items := slice.GetSlice()
			restore = func() error { return slice.Replace(items) }
		}
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("--%s=%s: %v", name, value, err)
		}
		t.Cleanup(func() {
			if err := restore(); err != nil {
				t.Errorf("restoring --%s: %v", name, err)
			}
			f.Changed = changed
		})
	}
}

// previewThen answers the first, revision-less call with a 409 preview and
// the confirmed one with the given response.
func previewThen(preview string, code int, payload string) func(*opStub) (int, string) {
	return func(st *opStub) (int, string) {
		if st.calls == 1 {
			return http.StatusConflict, preview
		}
		return code, payload
	}
}

func stderrOf(cmd *cobra.Command) string {
	return cmd.ErrOrStderr().(*bytes.Buffer).String()
}

func TestGatewayCommandTree(t *testing.T) {
	for parent, want := range map[*cobra.Command][]string{
		gatewayCmd:          {"list", "get", "create", "update", "delete", "status", "reenroll", "operation", "network"},
		gatewayOperationCmd: {"list", "get"},
		gatewayNetworkCmd:   {"list", "get", "attach", "detach", "update"},
	} {
		have := map[string]bool{}
		for _, c := range parent.Commands() {
			have[c.Name()] = true
		}
		for _, name := range want {
			if !have[name] {
				t.Errorf("%s is missing subcommand %q", parent.Name(), name)
			}
		}
	}
	if gatewayCmd.Parent() != Cmd {
		t.Error("gateway is not registered under compute")
	}
}

func TestGatewayListRenders(t *testing.T) {
	page := `{"count":1,"next":null,"previous":null,"results":[` + gatewayJSON + `]}`
	st, out, err := runOp(t, gatewayListCmd, nil, okOp(page), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/gateways/" {
		t.Errorf("path = %q", st.path)
	}
	for _, want := range []string{"edge", "active", "192.0.2.10", "<none>"} {
		if !strings.Contains(out, want) {
			t.Errorf("table %q is missing %q", out, want)
		}
	}

	_, out, err = runOp(t, gatewayListCmd, nil, okOp(page), "json", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got) != 1 || got[0]["name"] != "edge" {
		t.Errorf("json = %q (err %v), want the gateway list", out, err)
	}
}

func TestGatewayCreateConfirmsBeforeBilling(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stdin     string
		force     bool
		wantCalls int
	}{
		{name: "declined", stdin: "n\n"},
		{name: "confirmed", stdin: "y\n", wantCalls: 1},
		{name: "forced", force: true, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setFlags(t, gatewayCreateCmd, map[string]string{"name": "edge"})
			st, out, err := runOp(t, gatewayCreateCmd, nil,
				func(*opStub) (int, string) { return http.StatusAccepted, gatewayJSON }, "table", tc.stdin, tc.force)
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			if st.calls != tc.wantCalls {
				t.Fatalf("API calls = %d, want %d", st.calls, tc.wantCalls)
			}
			if !tc.force && !strings.Contains(stderrOf(gatewayCreateCmd), "billed") {
				t.Errorf("prompt %q does not say the gateway is billed", stderrOf(gatewayCreateCmd))
			}
			if tc.wantCalls == 0 {
				if out != "" {
					t.Errorf("cancelled create wrote %q", out)
				}
				return
			}
			if _, has := st.body["ipv4"]; st.body["name"] != "edge" || has {
				t.Errorf("body = %v, want the name and no ipv4", st.body)
			}
			if !strings.Contains(out, "5") {
				t.Errorf("output %q does not name the new gateway", out)
			}
		})
	}
}

func TestGatewayCreateSendsAddressChoices(t *testing.T) {
	setFlags(t, gatewayCreateCmd, map[string]string{"name": "edge", "ipv4": "192.0.2.10", "ipv6": "true"})
	st, _, err := runOp(t, gatewayCreateCmd, nil,
		func(*opStub) (int, string) { return http.StatusAccepted, gatewayJSON }, "json", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.body["ipv4"] != "192.0.2.10" || st.body["ipv6"] != true {
		t.Errorf("body = %v, want ipv4 and ipv6", st.body)
	}
}

func TestGatewayUpdateSendsOnlyWhatChanged(t *testing.T) {
	setFlags(t, gatewayUpdateCmd, map[string]string{"maintenance-hour": "3"})
	st, _, err := runOp(t, gatewayUpdateCmd, []string{"5"}, okOp(gatewayJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodPatch || st.path != "/api/cloud/gateways/5/" {
		t.Errorf("request = %s %s", st.method, st.path)
	}
	if len(st.body) != 1 || st.body["maintenance_hour"] != float64(3) {
		t.Errorf("body = %v, want only maintenance_hour", st.body)
	}
}

func TestGatewayUpdateRequiresAChange(t *testing.T) {
	st, _, err := runOp(t, gatewayUpdateCmd, []string{"5"}, okOp(gatewayJSON), "table", "", false)
	if err == nil || st.calls != 0 {
		t.Fatalf("err = %v, calls = %d; want a usage error before any call", err, st.calls)
	}
}

// A 200 that echoes the old value applied nothing.
func TestGatewayUpdateFailsWhenNotApplied(t *testing.T) {
	setFlags(t, gatewayUpdateCmd, map[string]string{"name": "other"})
	_, out, err := runOp(t, gatewayUpdateCmd, []string{"5"}, okOp(gatewayJSON), "table", "", false)
	if err == nil || out != "" {
		t.Fatalf("out = %q, err = %v; want an error", out, err)
	}
}

func TestGatewayDeleteConfirmsThePreview(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stdin     string
		force     bool
		wantCalls int
	}{
		{name: "declined", stdin: "n\n", wantCalls: 1},
		{name: "EOF", wantCalls: 1},
		{name: "confirmed", stdin: "yes\n", wantCalls: 2},
		{name: "forced", force: true, wantCalls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st, out, err := runOp(t, gatewayDeleteCmd, []string{"5"},
				previewThen(gatewayDeletePreviewJSON, http.StatusAccepted, gatewayOperationJSON), "table", tc.stdin, tc.force)
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			if st.calls != tc.wantCalls {
				t.Fatalf("API calls = %d, want %d", st.calls, tc.wantCalls)
			}
			prompt := stderrOf(gatewayDeleteCmd)
			if tc.force {
				if prompt != "" {
					t.Errorf("--force prompted: %q", prompt)
				}
			} else {
				for _, want := range []string{"web1", "net-a", "[y/N]"} {
					if !strings.Contains(prompt, want) {
						t.Errorf("prompt %q is missing %q", prompt, want)
					}
				}
			}
			if tc.wantCalls == 1 {
				if out != "" {
					t.Errorf("cancelled delete wrote %q", out)
				}
				return
			}
			if st.method != http.MethodDelete || st.path != "/api/cloud/gateways/5/" {
				t.Errorf("request = %s %s", st.method, st.path)
			}
			if st.query != "networks=3&revision=7&servers=42" {
				t.Errorf("query = %q, want the preview sent back", st.query)
			}
			if !strings.Contains(out, "op-1") {
				t.Errorf("output %q does not name the operation", out)
			}
		})
	}
}

// A 409 that carries no preview is a refusal, not a question.
func TestGatewayDeleteSurfacesARefusal(t *testing.T) {
	st, out, err := runOp(t, gatewayDeleteCmd, []string{"5"},
		func(*opStub) (int, string) { return http.StatusConflict, gatewayBusyJSON }, "table", "y\n", false)
	if err == nil || !strings.Contains(err.Error(), "Another operation is running.") {
		t.Fatalf("err = %v, want the API's refusal", err)
	}
	if st.calls != 1 || out != "" || stderrOf(gatewayDeleteCmd) != "" {
		t.Errorf("calls = %d, out = %q, prompt = %q", st.calls, out, stderrOf(gatewayDeleteCmd))
	}
}

func TestGatewayReenrollSendsThePreviewInTheBody(t *testing.T) {
	st, out, err := runOp(t, gatewayReenrollCmd, []string{"5"},
		previewThen(gatewayDeletePreviewJSON, http.StatusAccepted, gatewayOperationJSON), "json", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.calls != 2 || st.path != "/api/cloud/gateways/5/reenroll/" {
		t.Fatalf("calls = %d, path = %q", st.calls, st.path)
	}
	want := map[string]any{"revision": float64(7), "servers": []any{float64(42)}, "networks": []any{float64(3)}}
	if got, _ := json.Marshal(st.body); string(got) != mustJSON(t, want) {
		t.Errorf("body = %s, want %s", got, mustJSON(t, want))
	}
	var op map[string]any
	if err := json.Unmarshal([]byte(out), &op); err != nil || op["id"] != "op-1" {
		t.Errorf("json = %q (err %v), want the operation", out, err)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGatewayStatusRenders(t *testing.T) {
	status := `{"state":"config_error","state_reason":"apply failed","state_changed_at":null,` +
		`"desired_revision":8,"applied_revision":7,"last_status_at":"2026-11-14T18:00:00Z",` +
		`"agent_version":"1.2.3","envoy_version":null,"image_version":null,"attempted_revision":8,` +
		`"local_ready":true,"clock_synchronized":true,` +
		`"apply_error":{"kind":"firewall_rule","id":9,"message":"bad rule","rollback":"succeeded"},` +
		`"interfaces":[{"slot":1,"name":"eth1","state":"up","addresses":["10.0.0.1/24"]}],` +
		`"conntrack":null,"egress_probe":null,"security_updates":null,"reboot_required":false,"uptime_s":60}`
	st, out, err := runOp(t, gatewayStatusCmd, []string{"5"}, okOp(status), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/gateways/5/status/" {
		t.Errorf("path = %q", st.path)
	}
	for _, want := range []string{"config_error", "apply failed", "1.2.3", "bad rule", "eth1", "10.0.0.1/24"} {
		if !strings.Contains(out, want) {
			t.Errorf("status %q is missing %q", out, want)
		}
	}
	if strings.Contains(out, "0x") || strings.Contains(out, "{") {
		t.Errorf("status prints a raw Go value: %q", out)
	}
}

func TestGatewayOperationGet(t *testing.T) {
	st, out, err := runOp(t, gatewayOperationGetCmd, []string{"5", "op-1"}, okOp(gatewayOperationJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/gateways/5/operations/op-1/" {
		t.Errorf("path = %q", st.path)
	}
	for _, want := range []string{"op-1", "config_delete", "queued"} {
		if !strings.Contains(out, want) {
			t.Errorf("operation %q is missing %q", out, want)
		}
	}
}

func TestGatewayNetworkAttach(t *testing.T) {
	setFlags(t, gatewayNetworkAttachCmd, map[string]string{"network": "3", "role": "router"})
	st, out, err := runOp(t, gatewayNetworkAttachCmd, []string{"5"},
		func(*opStub) (int, string) { return http.StatusAccepted, gatewayOperationJSON }, "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/gateways/5/networks/" || st.body["network"] != float64(3) || st.body["role"] != "router" {
		t.Errorf("request = %s %v", st.path, st.body)
	}
	if !strings.Contains(out, "op-1") {
		t.Errorf("output %q does not name the operation", out)
	}
}

func TestGatewayNetworkAttachRejectsUnknownRole(t *testing.T) {
	setFlags(t, gatewayNetworkAttachCmd, map[string]string{"network": "3", "role": "bridge"})
	st, _, err := runOp(t, gatewayNetworkAttachCmd, []string{"5"}, okOp(gatewayOperationJSON), "table", "", false)
	if err == nil || st.calls != 0 {
		t.Fatalf("err = %v, calls = %d; want a usage error before any call", err, st.calls)
	}
}

func TestGatewayNetworkDetachSendsThePreviewBack(t *testing.T) {
	st, _, err := runOp(t, gatewayNetworkDetachCmd, []string{"5", "3"},
		previewThen(gatewayPreviewJSON, http.StatusAccepted, gatewayOperationJSON), "table", "y\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.calls != 2 || st.method != http.MethodDelete || st.path != "/api/cloud/gateways/5/networks/3/" {
		t.Fatalf("calls = %d, request = %s %s", st.calls, st.method, st.path)
	}
	if st.query != "revision=7&servers=42" {
		t.Errorf("query = %q, want the preview sent back", st.query)
	}
	if !strings.Contains(stderrOf(gatewayNetworkDetachCmd), "web1") {
		t.Errorf("prompt %q does not name the server", stderrOf(gatewayNetworkDetachCmd))
	}
}

func TestGatewayNetworkUpdateSNAT(t *testing.T) {
	off := strings.Replace(gatewayNetworkJSON, `"snat":true`, `"snat":false`, 1)

	t.Run("off goes through the preview", func(t *testing.T) {
		setFlags(t, gatewayNetworkUpdateCmd, map[string]string{"snat": "false"})
		st, out, err := runOp(t, gatewayNetworkUpdateCmd, []string{"5", "3"},
			previewThen(gatewayPreviewJSON, http.StatusOK, off), "table", "y\n", false)
		if err != nil {
			t.Fatalf("RunE: %v", err)
		}
		if st.calls != 2 || st.query != "revision=7&servers=42" || st.body["snat"] != false {
			t.Errorf("calls = %d, query = %q, body = %v", st.calls, st.query, st.body)
		}
		if !strings.Contains(out, "SNAT") {
			t.Errorf("output %q does not report SNAT", out)
		}
	})

	t.Run("on needs no confirmation", func(t *testing.T) {
		setFlags(t, gatewayNetworkUpdateCmd, map[string]string{"snat": "true"})
		st, _, err := runOp(t, gatewayNetworkUpdateCmd, []string{"5", "3"}, okOp(gatewayNetworkJSON), "table", "", false)
		if err != nil {
			t.Fatalf("RunE: %v", err)
		}
		if st.calls != 1 || st.query != "" || stderrOf(gatewayNetworkUpdateCmd) != "" {
			t.Errorf("calls = %d, query = %q, prompt = %q", st.calls, st.query, stderrOf(gatewayNetworkUpdateCmd))
		}
	})

	t.Run("a 200 that kept the old value is an error", func(t *testing.T) {
		setFlags(t, gatewayNetworkUpdateCmd, map[string]string{"snat": "false"})
		_, out, err := runOp(t, gatewayNetworkUpdateCmd, []string{"5", "3"}, okOp(gatewayNetworkJSON), "table", "", true)
		if err == nil || out != "" {
			t.Fatalf("out = %q, err = %v; want an error", out, err)
		}
	})

	t.Run("requires --snat", func(t *testing.T) {
		st, _, err := runOp(t, gatewayNetworkUpdateCmd, []string{"5", "3"}, okOp(gatewayNetworkJSON), "table", "", false)
		if err == nil || st.calls != 0 {
			t.Fatalf("err = %v, calls = %d; want a usage error before any call", err, st.calls)
		}
	})
}

const routeStatusJSON = `{"network":3,"gateway":5,"servers":[{"server":42,"hostname":"web1","power":"running",` +
	`"config":"missing","pending":"ok","routes_here":true,"acknowledged":false}],` +
	`"operations":[` + gatewayOperationJSON + `]}`

func TestNetworkRouteStatusRenders(t *testing.T) {
	st, out, err := runOp(t, networkRouteStatusCmd, []string{"3"}, okOp(routeStatusJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/private-networks/3/route-status/" {
		t.Errorf("path = %q", st.path)
	}
	for _, want := range []string{"web1", "running", "missing", "op-1", "Gateway:", "5"} {
		if !strings.Contains(out, want) {
			t.Errorf("status %q is missing %q", out, want)
		}
	}
	if strings.Contains(out, "0x") || strings.Contains(out, "{") {
		t.Errorf("status prints a raw Go value: %q", out)
	}
}

func TestNetworkApplyRouteConfirmsTheReboot(t *testing.T) {
	accepted := func(*opStub) (int, string) { return http.StatusAccepted, gatewayOperationJSON }
	for _, tc := range []struct {
		name      string
		stdin     string
		force     bool
		wantCalls int
	}{
		{name: "declined", stdin: "n\n"},
		{name: "confirmed", stdin: "y\n", wantCalls: 1},
		{name: "forced", force: true, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setFlags(t, networkApplyRouteCmd, map[string]string{"servers": "42,43"})
			st, out, err := runOp(t, networkApplyRouteCmd, []string{"3"}, accepted, "table", tc.stdin, tc.force)
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			if st.calls != tc.wantCalls {
				t.Fatalf("API calls = %d, want %d", st.calls, tc.wantCalls)
			}
			if !tc.force && !strings.Contains(stderrOf(networkApplyRouteCmd), "reboot") {
				t.Errorf("prompt %q does not say the servers reboot", stderrOf(networkApplyRouteCmd))
			}
			if tc.wantCalls == 0 {
				if out != "" {
					t.Errorf("cancelled apply wrote %q", out)
				}
				return
			}
			if st.path != "/api/cloud/private-networks/3/apply-route/" || bodyJSON(t, st) != `{"servers":[42,43]}` {
				t.Errorf("request = %s %s", st.path, bodyJSON(t, st))
			}
			if !strings.Contains(out, "op-1") {
				t.Errorf("output %q does not name the operation", out)
			}
		})
	}
}

func TestServerRename(t *testing.T) {
	renamed := mustJSON(t, newTestServerDetail(nil))
	renamed = strings.Replace(renamed, "vm.example.com", "web1.example.com", 1)

	t.Run("confirms, then sends only the hostname", func(t *testing.T) {
		st, out, err := runOp(t, serverRenameCmd, []string{"42", "web1.example.com"}, okOp(renamed), "table", "y\n", false)
		if err != nil {
			t.Fatalf("RunE: %v", err)
		}
		if st.method != http.MethodPatch || st.path != "/api/cloud/servers/42/" {
			t.Errorf("request = %s %s", st.method, st.path)
		}
		if got, want := bodyJSON(t, st), `{"hostname":"web1.example.com"}`; got != want {
			t.Errorf("body = %s, want %s", got, want)
		}
		if !strings.Contains(stderrOf(serverRenameCmd), "reboot") {
			t.Errorf("prompt %q does not say when the name takes effect", stderrOf(serverRenameCmd))
		}
		if !strings.Contains(out, "web1.example.com") {
			t.Errorf("output %q does not name the server", out)
		}
	})

	t.Run("a trailing dot is not part of the name", func(t *testing.T) {
		st, _, err := runOp(t, serverRenameCmd, []string{"42", "web1.example.com."}, okOp(renamed), "table", "", true)
		if err != nil || st.body["hostname"] != "web1.example.com" {
			t.Fatalf("err = %v, body = %v", err, st.body)
		}
	})

	t.Run("declined", func(t *testing.T) {
		st, out, err := runOp(t, serverRenameCmd, []string{"42", "web1.example.com"}, okOp(renamed), "table", "n\n", false)
		if err != nil || st.calls != 0 || out != "" {
			t.Fatalf("err = %v, calls = %d, out = %q", err, st.calls, out)
		}
	})

	t.Run("a 200 that kept the old name is an error", func(t *testing.T) {
		_, out, err := runOp(t, serverRenameCmd, []string{"42", "other.example.com"}, okOp(renamed), "table", "", true)
		if err == nil || out != "" {
			t.Fatalf("out = %q, err = %v; want an error", out, err)
		}
	})
}
