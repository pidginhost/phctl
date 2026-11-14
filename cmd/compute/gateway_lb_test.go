package compute

import (
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const (
	gatewayPoolJSON = `{"id":2,"name":"web","policy":"round_robin","proxy_protocol":false,` +
		`"health_check":{"type":"http","path":"/healthz","status_min":200,"status_max":399,"interval_s":5,` +
		`"timeout_s":2,"healthy_threshold":2,"unhealthy_threshold":3,"port":null},"changed_revision":8,"pending":false}`

	gatewayBackendJSON = `{"id":6,"network":3,"address":"10.0.0.5","port":8080,"weight":1,"enabled":true,` +
		`"health":"healthy","health_changed":null,"changed_revision":8,"pending":false}`

	gatewayListenerJSON = `{"id":7,"protocol":"tcp","port":80,"mode":"http","pool":null,` +
		`"allowed_sources":["198.51.100.0/24"],"enabled":true,"comment":"","changed_revision":8,"pending":true}`

	gatewayRouteJSON = `{"id":8,"hostnames":["example.test"],"path_prefix":"/","pool":2,"redirect_https":false,` +
		`"enabled":true,"changed_revision":8,"pending":false}`
)

func page(item string) string {
	return `{"count":1,"next":null,"previous":null,"results":[` + item + `]}`
}

func TestGatewayLBCommandTree(t *testing.T) {
	for _, parent := range []*cobra.Command{gatewayPoolCmd, gatewayBackendCmd, gatewayListenerCmd, gatewayRouteCmd} {
		have := map[string]bool{}
		for _, c := range parent.Commands() {
			have[c.Name()] = true
		}
		for _, name := range []string{"list", "get", "create", "update", "delete"} {
			if !have[name] {
				t.Errorf("%s is missing subcommand %q", parent.Name(), name)
			}
		}
		if parent.Parent() != gatewayCmd {
			t.Errorf("%s is not registered under gateway", parent.Name())
		}
	}
}

// Every read and delete must reach its own URL: the SDK takes the nested IDs
// in an order that differs from the path's, so a swap compiles and misroutes.
func TestGatewayLBRoutes(t *testing.T) {
	for _, tc := range []struct {
		cmd      *cobra.Command
		args     []string
		respond  func(*opStub) (int, string)
		method   string
		path     string
		wantText string
	}{
		{gatewayPoolListCmd, []string{"5"}, okOp(page(gatewayPoolJSON)), http.MethodGet,
			"/api/cloud/gateways/5/pools/", "/healthz"},
		{gatewayPoolGetCmd, []string{"5", "2"}, okOp(gatewayPoolJSON), http.MethodGet,
			"/api/cloud/gateways/5/pools/2/", "round_robin"},
		{gatewayPoolDeleteCmd, []string{"5", "2"}, previewThen(gatewayPreviewJSON, http.StatusAccepted, gatewayOperationJSON),
			http.MethodDelete, "/api/cloud/gateways/5/pools/2/", "op-1"},
		{gatewayBackendListCmd, []string{"5", "2"}, okOp(page(gatewayBackendJSON)), http.MethodGet,
			"/api/cloud/gateways/5/pools/2/backends/", "healthy"},
		{gatewayBackendGetCmd, []string{"5", "2", "6"}, okOp(gatewayBackendJSON), http.MethodGet,
			"/api/cloud/gateways/5/pools/2/backends/6/", "10.0.0.5"},
		{gatewayBackendDeleteCmd, []string{"5", "2", "6"}, previewThen(gatewayPreviewJSON, http.StatusAccepted, gatewayOperationJSON),
			http.MethodDelete, "/api/cloud/gateways/5/pools/2/backends/6/", "op-1"},
		{gatewayListenerListCmd, []string{"5"}, okOp(page(gatewayListenerJSON)), http.MethodGet,
			"/api/cloud/gateways/5/listeners/", "198.51.100.0/24"},
		{gatewayListenerGetCmd, []string{"5", "7"}, okOp(gatewayListenerJSON), http.MethodGet,
			"/api/cloud/gateways/5/listeners/7/", "http"},
		{gatewayListenerDeleteCmd, []string{"5", "7"}, previewThen(gatewayPreviewJSON, http.StatusAccepted, gatewayOperationJSON),
			http.MethodDelete, "/api/cloud/gateways/5/listeners/7/", "op-1"},
		{gatewayRouteListCmd, []string{"5", "7"}, okOp(page(gatewayRouteJSON)), http.MethodGet,
			"/api/cloud/gateways/5/listeners/7/routes/", "example.test"},
		{gatewayRouteGetCmd, []string{"5", "7", "8"}, okOp(gatewayRouteJSON), http.MethodGet,
			"/api/cloud/gateways/5/listeners/7/routes/8/", "example.test"},
		{gatewayRouteDeleteCmd, []string{"5", "7", "8"}, previewThen(gatewayPreviewJSON, http.StatusAccepted, gatewayOperationJSON),
			http.MethodDelete, "/api/cloud/gateways/5/listeners/7/routes/8/", "op-1"},
	} {
		t.Run(tc.cmd.CommandPath(), func(t *testing.T) {
			st, out, err := runOp(t, tc.cmd, tc.args, tc.respond, "table", "", true)
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			if st.method != tc.method || st.path != tc.path {
				t.Errorf("request = %s %s, want %s %s", st.method, st.path, tc.method, tc.path)
			}
			if tc.method == http.MethodDelete && (st.calls != 2 || st.query != "revision=7") {
				t.Errorf("calls = %d, query = %q; want the preview's revision sent back", st.calls, st.query)
			}
			if !strings.Contains(out, tc.wantText) {
				t.Errorf("output %q is missing %q", out, tc.wantText)
			}
			if strings.Contains(out, "0x") || strings.Contains(out, "{") {
				t.Errorf("output prints a raw Go value: %q", out)
			}
		})
	}
}

func TestGatewayLBWrites(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cmd      *cobra.Command
		args     []string
		flags    map[string]string
		response string
		method   string
		path     string
		body     string
	}{
		{
			name: "pool create", cmd: gatewayPoolCreateCmd, args: []string{"5"},
			flags: map[string]string{"name": "web", "policy": "round_robin", "health-check": "http",
				"health-path": "/healthz", "health-interval": "5", "health-port": "0"},
			response: gatewayPoolJSON, method: http.MethodPost, path: "/api/cloud/gateways/5/pools/",
			body: `{"health_check":{"interval_s":5,"path":"/healthz","port":null,"type":"http"},"name":"web","policy":"round_robin"}`,
		},
		{
			name: "pool update", cmd: gatewayPoolUpdateCmd, args: []string{"5", "2"},
			flags:    map[string]string{"health-timeout": "2"},
			response: gatewayPoolJSON, method: http.MethodPatch, path: "/api/cloud/gateways/5/pools/2/",
			body: `{"health_check":{"timeout_s":2}}`,
		},
		{
			name: "backend create", cmd: gatewayBackendCreateCmd, args: []string{"5", "2"},
			flags:    map[string]string{"network": "3", "address": "10.0.0.5", "port": "8080"},
			response: gatewayBackendJSON, method: http.MethodPost, path: "/api/cloud/gateways/5/pools/2/backends/",
			body: `{"address":"10.0.0.5","network":3,"port":8080}`,
		},
		{
			name: "backend update", cmd: gatewayBackendUpdateCmd, args: []string{"5", "2", "6"},
			flags:    map[string]string{"enabled": "true", "weight": "1"},
			response: gatewayBackendJSON, method: http.MethodPatch, path: "/api/cloud/gateways/5/pools/2/backends/6/",
			body: `{"enabled":true,"weight":1}`,
		},
		{
			name: "listener create", cmd: gatewayListenerCreateCmd, args: []string{"5"},
			flags:    map[string]string{"protocol": "tcp", "port": "80", "mode": "http", "allowed-sources": "198.51.100.0/24"},
			response: gatewayListenerJSON, method: http.MethodPost, path: "/api/cloud/gateways/5/listeners/",
			body: `{"allowed_sources":["198.51.100.0/24"],"mode":"http","port":80,"protocol":"tcp"}`,
		},
		{
			name: "listener update", cmd: gatewayListenerUpdateCmd, args: []string{"5", "7"},
			flags:    map[string]string{"allowed-sources": "", "comment": ""},
			response: gatewayListenerJSON, method: http.MethodPatch, path: "/api/cloud/gateways/5/listeners/7/",
			body: `{"allowed_sources":[],"comment":""}`,
		},
		{
			name: "route create", cmd: gatewayRouteCreateCmd, args: []string{"5", "7"},
			flags:    map[string]string{"hostnames": "example.test", "path-prefix": "/", "pool": "2"},
			response: gatewayRouteJSON, method: http.MethodPost, path: "/api/cloud/gateways/5/listeners/7/routes/",
			body: `{"hostnames":["example.test"],"path_prefix":"/","pool":2}`,
		},
		{
			name: "route update", cmd: gatewayRouteUpdateCmd, args: []string{"5", "7", "8"},
			flags:    map[string]string{"pool": "2", "hostnames": ""},
			response: gatewayRouteJSON, method: http.MethodPatch, path: "/api/cloud/gateways/5/listeners/7/routes/8/",
			body: `{"hostnames":[],"pool":2}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setFlags(t, tc.cmd, tc.flags)
			code := http.StatusOK
			if tc.method == http.MethodPost {
				code = http.StatusCreated
			}
			st, out, err := runOp(t, tc.cmd, tc.args,
				func(*opStub) (int, string) { return code, tc.response }, "table", "", false)
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			if st.method != tc.method || st.path != tc.path {
				t.Errorf("request = %s %s, want %s %s", st.method, st.path, tc.method, tc.path)
			}
			if got := bodyJSON(t, st); got != tc.body {
				t.Errorf("body = %s\nwant   %s", got, tc.body)
			}
			if out == "" {
				t.Error("no output")
			}
		})
	}
}

func TestGatewayLBUpdatesRequireAChange(t *testing.T) {
	for cmd, args := range map[*cobra.Command][]string{
		gatewayPoolUpdateCmd:     {"5", "2"},
		gatewayBackendUpdateCmd:  {"5", "2", "6"},
		gatewayListenerUpdateCmd: {"5", "7"},
		gatewayRouteUpdateCmd:    {"5", "7", "8"},
	} {
		t.Run(cmd.CommandPath(), func(t *testing.T) {
			st, _, err := runOp(t, cmd, args, okOp(`{}`), "table", "", false)
			if err == nil || st.calls != 0 {
				t.Fatalf("err = %v, calls = %d; want a usage error before any call", err, st.calls)
			}
		})
	}
}

func TestGatewayLBRejectsBadInput(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cmd   *cobra.Command
		args  []string
		flags map[string]string
	}{
		{"pool policy", gatewayPoolCreateCmd, []string{"5"}, map[string]string{"name": "web", "policy": "random"}},
		{"health check type", gatewayPoolCreateCmd, []string{"5"}, map[string]string{"name": "web", "health-check": "ping"}},
		{"listener mode", gatewayListenerCreateCmd, []string{"5"}, map[string]string{"protocol": "tcp", "port": "80", "mode": "quic"}},
		{"listener protocol", gatewayListenerCreateCmd, []string{"5"}, map[string]string{"protocol": "sctp", "port": "80", "mode": "tcp"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setFlags(t, tc.cmd, tc.flags)
			st, out, err := runOp(t, tc.cmd, tc.args, created(`{}`), "table", "", false)
			if err == nil || st.calls != 0 || out != "" {
				t.Fatalf("err = %v, calls = %d, out = %q; want an error before any call", err, st.calls, out)
			}
		})
	}
}

// A 200 that echoes the old value applied nothing.
func TestGatewayBackendUpdateFailsWhenNotApplied(t *testing.T) {
	setFlags(t, gatewayBackendUpdateCmd, map[string]string{"enabled": "false"})
	_, out, err := runOp(t, gatewayBackendUpdateCmd, []string{"5", "2", "6"}, okOp(gatewayBackendJSON), "table", "", false)
	if err == nil || !strings.Contains(err.Error(), "enabled") || out != "" {
		t.Fatalf("out = %q, err = %v; want an error naming enabled", out, err)
	}
}
