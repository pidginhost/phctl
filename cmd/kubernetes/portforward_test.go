package kubernetes

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const portForwardJSON = `{"id":9,"internal_ip":"10.0.0.50","port":8080,"protocol":"tcp"}`

func TestKubernetesExposesPortForward(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}
	if !names["port-forward"] {
		t.Fatal("kubernetes is missing the port-forward command")
	}
}

func TestPortForwardSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range portForwardCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get", "create", "update", "delete"} {
		if !names[want] {
			t.Errorf("port-forward is missing subcommand %q", want)
		}
	}
	for _, name := range []string{"internal-ip", "port", "protocol"} {
		if portForwardCreateCmd.Flags().Lookup(name) == nil {
			t.Errorf("port-forward create missing flag --%s", name)
		}
		if portForwardUpdateCmd.Flags().Lookup(name) == nil {
			t.Errorf("port-forward update missing flag --%s", name)
		}
	}
}

func TestPortForwardListPaginates(t *testing.T) {
	rec, out, _, err := runCmd(t, portForwardListCmd, []string{"42"}, func(_ apiCall, n int) (int, string) {
		if n == 0 {
			return http.StatusOK, `{"count":2,"next":"https://api.test/next","previous":null,"results":[` + portForwardJSON + `]}`
		}
		second := strings.Replace(portForwardJSON, `"port":8080`, `"port":9090`, 1)
		return http.StatusOK, `{"count":2,"next":null,"previous":null,"results":[` + second + `]}`
	}, "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if rec.count() != 2 {
		t.Fatalf("expected pagination to be followed, got %d call(s)", rec.count())
	}
	if rec.calls[0].path != "/api/kubernetes/clusters/42/port-forwards/" {
		t.Errorf("path = %q", rec.calls[0].path)
	}
	for _, want := range []string{"10.0.0.50", "8080", "9090", "tcp"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPortForwardGetRendersForward(t *testing.T) {
	rec, out, _, err := runCmd(t, portForwardGetCmd, []string{"42", "9"}, okBody(portForwardJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if rec.last().path != "/api/kubernetes/clusters/42/port-forwards/9/" {
		t.Errorf("path = %q", rec.last().path)
	}
	if !strings.Contains(out, "10.0.0.50") {
		t.Errorf("output missing the internal IP:\n%s", out)
	}
}

func TestPortForwardGetRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	for _, body := range []string{"", "null"} {
		t.Run("body="+body, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("empty 200 body caused a panic: %v", r)
				}
			}()
			_, _, _, err := runCmd(t, portForwardGetCmd, []string{"42", "9"}, okBody(body), "table", "", false)
			if err == nil {
				t.Fatal("expected an error when the server returns no port forward")
			}
		})
	}
}

func TestPortForwardCreateSendsForward(t *testing.T) {
	setFlags(t, portForwardCreateCmd, map[string]string{"internal-ip": "10.0.0.50", "port": "8080", "protocol": "tcp"})
	rec, out, _, err := runCmd(t, portForwardCreateCmd, []string{"42"},
		reply(http.StatusCreated, portForwardJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPost || call.path != "/api/kubernetes/clusters/42/port-forwards/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if call.body["internal_ip"] != "10.0.0.50" || call.body["port"] != float64(8080) || call.body["protocol"] != "tcp" {
		t.Errorf("body = %v", call.body)
	}
	if !strings.Contains(out, "9") {
		t.Errorf("output missing the new ID:\n%s", out)
	}
}

func TestPortForwardCreateRequiresInternalIPAndPort(t *testing.T) {
	for name, flags := range map[string]map[string]string{
		"no-ip":   {"port": "8080"},
		"no-port": {"internal-ip": "10.0.0.50"},
	} {
		t.Run(name, func(t *testing.T) {
			setFlags(t, portForwardCreateCmd, flags)
			rec, _, _, err := runCmd(t, portForwardCreateCmd, []string{"42"},
				reply(http.StatusCreated, portForwardJSON), "table", "", false)
			if err == nil {
				t.Fatal("expected an error for an incomplete port forward")
			}
			if rec.count() != 0 {
				t.Errorf("incomplete create still reached the API (%d call(s))", rec.count())
			}
		})
	}
}

func TestPortForwardRejectsUnknownProtocol(t *testing.T) {
	setFlags(t, portForwardCreateCmd, map[string]string{"internal-ip": "10.0.0.50", "port": "8080", "protocol": "sctp"})
	rec, _, _, err := runCmd(t, portForwardCreateCmd, []string{"42"},
		reply(http.StatusCreated, portForwardJSON), "table", "", false)
	if err == nil {
		t.Fatal("expected an error for a protocol outside the schema enum")
	}
	if rec.count() != 0 {
		t.Errorf("invalid protocol still reached the API (%d call(s))", rec.count())
	}
}

func TestPortForwardUpdateSendsOnlyChangedFields(t *testing.T) {
	setFlags(t, portForwardUpdateCmd, map[string]string{"port": "9090"})
	rec, _, _, err := runCmd(t, portForwardUpdateCmd, []string{"42", "9"},
		okBody(strings.Replace(portForwardJSON, `"port":8080`, `"port":9090`, 1)), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPatch || call.path != "/api/kubernetes/clusters/42/port-forwards/9/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if call.body["port"] != float64(9090) {
		t.Errorf("body[port] = %v", call.body["port"])
	}
	for _, key := range []string{"internal_ip", "protocol"} {
		if _, present := call.body[key]; present {
			t.Errorf("body carries unset field %q: %v", key, call.body)
		}
	}
}

func TestPortForwardUpdateRequiresAField(t *testing.T) {
	rec, _, _, err := runCmd(t, portForwardUpdateCmd, []string{"42", "9"}, okBody(portForwardJSON), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when no field flags are given")
	}
	if rec.count() != 0 {
		t.Errorf("empty update still reached the API (%d call(s))", rec.count())
	}
}

func TestPortForwardUpdateRejectsUnappliedChange(t *testing.T) {
	setFlags(t, portForwardUpdateCmd, map[string]string{"port": "9090"})
	_, _, _, err := runCmd(t, portForwardUpdateCmd, []string{"42", "9"}, okBody(portForwardJSON), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the forward still reports the old port")
	}
}

func TestPortForwardDeleteConfirmsFirst(t *testing.T) {
	rec, _, errOut, err := runCmd(t, portForwardDeleteCmd, []string{"42", "9"},
		reply(http.StatusNoContent, ""), "table", "n\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if rec.count() != 0 {
		t.Errorf("declined delete still reached the API (%d call(s))", rec.count())
	}
	if !strings.Contains(errOut, "[y/N]") {
		t.Errorf("no confirmation prompt: %q", errOut)
	}
}

func TestPortForwardDeleteWithForce(t *testing.T) {
	rec, _, _, err := runCmd(t, portForwardDeleteCmd, []string{"42", "9"},
		reply(http.StatusNoContent, ""), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodDelete || call.path != "/api/kubernetes/clusters/42/port-forwards/9/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
}

func TestPortForwardListRendersJSON(t *testing.T) {
	_, out, _, err := runCmd(t, portForwardListCmd, []string{"42"},
		okBody(`{"count":1,"next":null,"previous":null,"results":[`+portForwardJSON+`]}`), "json", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(got) != 1 || got[0]["internal_ip"] != "10.0.0.50" {
		t.Errorf("unexpected payload: %v", got)
	}
}

// --- node metrics (unblocked: the counters are int64 now) ---

func TestNodeMetricsRendersTable(t *testing.T) {
	rec, out, _, err := runCmd(t, nodeMetricsCmd, []string{"42", "3", "11"},
		okBody(`{"cpu":0.25,"mem":4294967296,"maxmem":8589934592,"status":"running","uptime":3600,`+
			`"netin":5000000000,"netout":6000000000,"disk":7000000000,"maxdisk":8000000000}`),
		"table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if rec.last().path != "/api/kubernetes/clusters/42/resource-pools/3/nodes/11/metrics/" {
		t.Errorf("path = %q", rec.last().path)
	}
	// Every one of these is past int32; they used to make the response undecodable.
	for _, want := range []string{"running", "4294967296", "8589934592", "5000000000", "7000000000", "8000000000"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestNodeMetricsRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 200 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, nodeMetricsCmd, []string{"42", "3", "11"}, okBody(""), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the server returns no metrics")
	}
}

func TestNodeRRDSendsTimeframe(t *testing.T) {
	setFlags(t, nodeRRDCmd, map[string]string{"timeframe": "week"})
	rec, _, _, err := runCmd(t, nodeRRDCmd, []string{"42", "3", "11"},
		okBody(`{"timeframe":"week","data":[]}`), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if got := rec.last().query.Get("timeframe"); got != "week" {
		t.Errorf("timeframe query = %q, want week", got)
	}
}

func TestNodeRRDRejectsUnknownTimeframe(t *testing.T) {
	setFlags(t, nodeRRDCmd, map[string]string{"timeframe": "fortnight"})
	rec, _, _, err := runCmd(t, nodeRRDCmd, []string{"42", "3", "11"},
		okBody(`{"timeframe":"hour","data":[]}`), "table", "", false)
	if err == nil {
		t.Fatal("expected an error: the API silently falls back instead of rejecting")
	}
	if rec.count() != 0 {
		t.Errorf("invalid timeframe still reached the API (%d call(s))", rec.count())
	}
}
