package compute

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const (
	gatewayRuleJSON = `{"id":9,"position":0,"action":"ACCEPT","source":{"network":3},` +
		`"destination":{"internet":true},"protocol":"tcp",` +
		`"ports":[{"start":80,"end":80},{"start":8000,"end":8100}],"enabled":true,"comment":"web",` +
		`"changed_revision":8,"pending":true}`

	gatewayForwardJSON = `{"id":4,"protocol":"tcp","public_ports":{"start":8080,"end":8090},` +
		`"allowed_sources":["198.51.100.0/24"],"target":{"network":3,"address":"10.0.0.5","port_start":80},` +
		`"preserve_client_ip":false,"enabled":true,"comment":"","changed_revision":8,"pending":false}`
)

func created(payload string) func(*opStub) (int, string) {
	return func(*opStub) (int, string) { return http.StatusCreated, payload }
}

func bodyJSON(t *testing.T, st *opStub) string {
	t.Helper()
	return mustJSON(t, st.body)
}

func TestGatewayRuleCommandTree(t *testing.T) {
	for parent, want := range map[*cobra.Command][]string{
		gatewayFirewallRuleCmd: {"list", "get", "create", "update", "delete", "move"},
		gatewayPortForwardCmd:  {"list", "get", "create", "update", "delete"},
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
		if parent.Parent() != gatewayCmd {
			t.Errorf("%s is not registered under gateway", parent.Name())
		}
	}
}

func TestGatewayFirewallRuleListRenders(t *testing.T) {
	page := `{"count":1,"next":null,"previous":null,"results":[` + gatewayRuleJSON + `]}`
	st, out, err := runOp(t, gatewayFirewallRuleListCmd, []string{"5"}, okOp(page), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/gateways/5/firewall-rules/" {
		t.Errorf("path = %q", st.path)
	}
	for _, want := range []string{"ACCEPT", "network:3", "internet", "tcp", "80,8000-8100", "web"} {
		if !strings.Contains(out, want) {
			t.Errorf("table %q is missing %q", out, want)
		}
	}
}

func TestGatewayFirewallRuleCreateSendsTheRule(t *testing.T) {
	setFlags(t, gatewayFirewallRuleCreateCmd, map[string]string{
		"action": "ACCEPT", "source": "network:3", "destination": "internet",
		"protocol": "tcp", "ports": "80,8000-8100", "comment": "web",
	})
	st, out, err := runOp(t, gatewayFirewallRuleCreateCmd, []string{"5"}, created(gatewayRuleJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodPost || st.path != "/api/cloud/gateways/5/firewall-rules/" {
		t.Errorf("request = %s %s", st.method, st.path)
	}
	want := `{"action":"ACCEPT","comment":"web","destination":{"internet":true},` +
		`"ports":[{"end":80,"start":80},{"end":8100,"start":8000}],"protocol":"tcp","source":{"network":3}}`
	if got := bodyJSON(t, st); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
	if !strings.Contains(out, "9") {
		t.Errorf("output %q does not name the rule", out)
	}
}

func TestGatewayFirewallRuleCreateSendsACIDR(t *testing.T) {
	setFlags(t, gatewayFirewallRuleCreateCmd, map[string]string{
		"action": "DROP", "source": "198.51.100.0/24", "destination": "10.0.0.0/24", "protocol": "any", "enabled": "false",
	})
	st, _, err := runOp(t, gatewayFirewallRuleCreateCmd, []string{"5"}, created(gatewayRuleJSON), "json", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	want := `{"action":"DROP","destination":{"cidr":"10.0.0.0/24"},"enabled":false,"protocol":"any","source":{"cidr":"198.51.100.0/24"}}`
	if got := bodyJSON(t, st); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
}

func TestGatewayFirewallRuleRejectsBadInput(t *testing.T) {
	valid := map[string]string{"action": "ACCEPT", "source": "network:3", "destination": "internet", "protocol": "tcp"}
	for name, override := range map[string][2]string{
		"internet is not a source":  {"source", "internet"},
		"network needs a number":    {"source", "network:x"},
		"unknown action":            {"action", "allow"},
		"unknown protocol":          {"protocol", "gre"},
		"port zero":                 {"ports", "0"},
		"port too large":            {"ports", "70000"},
		"range backwards":           {"ports", "90-80"},
		"port is not a number":      {"ports", "http"},
		"missing a required option": {"action", ""},
	} {
		t.Run(name, func(t *testing.T) {
			flags := map[string]string{}
			for k, v := range valid {
				flags[k] = v
			}
			if override[1] == "" {
				delete(flags, override[0])
			} else {
				flags[override[0]] = override[1]
			}
			setFlags(t, gatewayFirewallRuleCreateCmd, flags)
			st, out, err := runOp(t, gatewayFirewallRuleCreateCmd, []string{"5"}, created(gatewayRuleJSON), "table", "", false)
			if err == nil || st.calls != 0 || out != "" {
				t.Fatalf("err = %v, calls = %d, out = %q; want an error before any call", err, st.calls, out)
			}
		})
	}
}

func TestGatewayFirewallRuleUpdateSendsOnlyWhatChanged(t *testing.T) {
	setFlags(t, gatewayFirewallRuleUpdateCmd, map[string]string{"enabled": "true", "ports": ""})
	st, _, err := runOp(t, gatewayFirewallRuleUpdateCmd, []string{"5", "9"}, okOp(gatewayRuleJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodPatch || st.path != "/api/cloud/gateways/5/firewall-rules/9/" {
		t.Errorf("request = %s %s", st.method, st.path)
	}
	// An empty --ports clears them, so it must reach the API as an empty list.
	if got, want := bodyJSON(t, st), `{"enabled":true,"ports":[]}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestGatewayFirewallRuleUpdateRequiresAChange(t *testing.T) {
	st, _, err := runOp(t, gatewayFirewallRuleUpdateCmd, []string{"5", "9"}, okOp(gatewayRuleJSON), "table", "", false)
	if err == nil || st.calls != 0 {
		t.Fatalf("err = %v, calls = %d; want a usage error before any call", err, st.calls)
	}
}

func TestGatewayFirewallRuleUpdateFailsWhenNotApplied(t *testing.T) {
	setFlags(t, gatewayFirewallRuleUpdateCmd, map[string]string{"action": "DROP"})
	_, out, err := runOp(t, gatewayFirewallRuleUpdateCmd, []string{"5", "9"}, okOp(gatewayRuleJSON), "table", "", false)
	if err == nil || !strings.Contains(err.Error(), "action") || out != "" {
		t.Fatalf("out = %q, err = %v; want an error naming action", out, err)
	}
}

func TestGatewayFirewallRuleDeleteSendsTheRevisionBack(t *testing.T) {
	st, out, err := runOp(t, gatewayFirewallRuleDeleteCmd, []string{"5", "9"},
		previewThen(gatewayPreviewJSON, http.StatusAccepted, gatewayOperationJSON), "table", "y\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.calls != 2 || st.method != http.MethodDelete || st.path != "/api/cloud/gateways/5/firewall-rules/9/" {
		t.Fatalf("calls = %d, request = %s %s", st.calls, st.method, st.path)
	}
	if st.query != "revision=7" {
		t.Errorf("query = %q, want the preview's revision", st.query)
	}
	if !strings.Contains(out, "op-1") {
		t.Errorf("output %q does not name the operation", out)
	}
}

func TestGatewayFirewallRuleMove(t *testing.T) {
	moved := strings.Replace(gatewayRuleJSON, `"position":0`, `"position":2`, 1)

	t.Run("sends the position and the revision", func(t *testing.T) {
		setFlags(t, gatewayFirewallRuleMoveCmd, map[string]string{"position": "2"})
		st, _, err := runOp(t, gatewayFirewallRuleMoveCmd, []string{"5", "9"},
			previewThen(gatewayPreviewJSON, http.StatusOK, moved), "table", "", true)
		if err != nil {
			t.Fatalf("RunE: %v", err)
		}
		if st.calls != 2 || st.path != "/api/cloud/gateways/5/firewall-rules/9/move/" || st.query != "revision=7" {
			t.Errorf("calls = %d, path = %q, query = %q", st.calls, st.path, st.query)
		}
		if st.body["position"] != float64(2) {
			t.Errorf("body = %v", st.body)
		}
	})

	t.Run("a rule that stayed put is an error", func(t *testing.T) {
		setFlags(t, gatewayFirewallRuleMoveCmd, map[string]string{"position": "2"})
		_, out, err := runOp(t, gatewayFirewallRuleMoveCmd, []string{"5", "9"},
			previewThen(gatewayPreviewJSON, http.StatusOK, gatewayRuleJSON), "table", "", true)
		if err == nil || out != "" {
			t.Fatalf("out = %q, err = %v; want an error", out, err)
		}
	})
}

func TestGatewayPortForwardListRenders(t *testing.T) {
	page := `{"count":1,"next":null,"previous":null,"results":[` + gatewayForwardJSON + `]}`
	st, out, err := runOp(t, gatewayPortForwardListCmd, []string{"5"}, okOp(page), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/gateways/5/port-forwards/" {
		t.Errorf("path = %q", st.path)
	}
	for _, want := range []string{"tcp", "8080-8090", "10.0.0.5:80", "198.51.100.0/24"} {
		if !strings.Contains(out, want) {
			t.Errorf("table %q is missing %q", out, want)
		}
	}

	_, out, err = runOp(t, gatewayPortForwardListCmd, []string{"5"}, okOp(page), "json", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got) != 1 || got[0]["id"] != float64(4) {
		t.Errorf("json = %q (err %v), want the forwards", out, err)
	}
}

func TestGatewayPortForwardCreateSendsTheForward(t *testing.T) {
	setFlags(t, gatewayPortForwardCreateCmd, map[string]string{
		"protocol": "tcp", "public-ports": "8080-8090", "target-network": "3",
		"target-address": "10.0.0.5", "target-port": "80", "allowed-sources": "198.51.100.0/24",
	})
	st, _, err := runOp(t, gatewayPortForwardCreateCmd, []string{"5"}, created(gatewayForwardJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	want := `{"allowed_sources":["198.51.100.0/24"],"protocol":"tcp","public_ports":{"end":8090,"start":8080},` +
		`"target":{"address":"10.0.0.5","network":3,"port_start":80}}`
	if got := bodyJSON(t, st); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
}

// The API replaces a target whole, so a partial one would blank the rest.
func TestGatewayPortForwardUpdateNeedsTheWholeTarget(t *testing.T) {
	setFlags(t, gatewayPortForwardUpdateCmd, map[string]string{"target-port": "81"})
	st, _, err := runOp(t, gatewayPortForwardUpdateCmd, []string{"5", "4"}, okOp(gatewayForwardJSON), "table", "", false)
	if err == nil || st.calls != 0 || !strings.Contains(err.Error(), "target") {
		t.Fatalf("err = %v, calls = %d; want a target error before any call", err, st.calls)
	}
}

func TestGatewayPortForwardUpdateSendsOnlyWhatChanged(t *testing.T) {
	setFlags(t, gatewayPortForwardUpdateCmd, map[string]string{"comment": "", "allowed-sources": ""})
	st, _, err := runOp(t, gatewayPortForwardUpdateCmd, []string{"5", "4"}, okOp(gatewayForwardJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodPatch || st.path != "/api/cloud/gateways/5/port-forwards/4/" {
		t.Errorf("request = %s %s", st.method, st.path)
	}
	if got, want := bodyJSON(t, st), `{"allowed_sources":[],"comment":""}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestGatewayPortForwardDeleteSendsTheRevisionBack(t *testing.T) {
	st, _, err := runOp(t, gatewayPortForwardDeleteCmd, []string{"5", "4"},
		previewThen(gatewayPreviewJSON, http.StatusAccepted, gatewayOperationJSON), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.calls != 2 || st.path != "/api/cloud/gateways/5/port-forwards/4/" || st.query != "revision=7" {
		t.Errorf("calls = %d, path = %q, query = %q", st.calls, st.path, st.query)
	}
}
