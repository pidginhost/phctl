package compute

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"
)

type opStub struct {
	method string
	path   string
	query  string
	body   map[string]interface{}
	calls  int
}

func parentOpCommand(t *testing.T, cmd *cobra.Command, format string, force bool) {
	t.Helper()

	root := &cobra.Command{Use: "phctl"}
	root.PersistentFlags().StringP("output", "o", format, "Output format")
	root.PersistentFlags().BoolP("force", "f", force, "Skip confirmation prompts")
	originalParent := cmd.Parent()
	if originalParent != nil {
		originalParent.RemoveCommand(cmd)
	}
	root.AddCommand(cmd)
	t.Cleanup(func() {
		root.RemoveCommand(cmd)
		if originalParent != nil {
			originalParent.AddCommand(cmd)
		}
	})
}

// runOp drives a command's RunE against a stub API. respond receives the stub
// so a handler can answer differently per call (pagination) or per method.
func runOp(t *testing.T, cmd *cobra.Command, args []string,
	respond func(*opStub) (int, string), format, stdin string, force bool) (*opStub, string, error) {
	t.Helper()

	st := &opStub{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st.calls++
		st.method = r.Method
		st.path = r.URL.Path
		st.query = r.URL.RawQuery
		st.body = nil
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&st.body)
		}
		code, payload := respond(st)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(server.Close)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	// Parent the real command under a throwaway root carrying the global
	// flags. Passing a stand-in child instead would hide the command's own
	// flags from its RunE, which is where several of these read their input.
	parentOpCommand(t, cmd, format, force)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader(stdin))

	// Run first: Go evaluates arguments left to right, so returning
	// out.String() alongside the call would capture the buffer before it fills.
	err := cmd.RunE(cmd, args)
	return st, out.String(), err
}

func okOp(payload string) func(*opStub) (int, string) {
	return func(*opStub) (int, string) { return http.StatusOK, payload }
}

// --- structure ---

func TestServerHasPhase2Subcommands(t *testing.T) {
	want := []string{"rescue", "boot-isos", "usage", "activity", "retry-provision", "public-interface"}
	have := map[string]bool{}
	for _, c := range serverCmd.Commands() {
		have[c.Name()] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("compute server is missing subcommand %q", w)
		}
	}
	rescue := map[string]bool{}
	for _, c := range serverRescueCmd.Commands() {
		rescue[c.Name()] = true
	}
	for _, w := range []string{"enter", "exit"} {
		if !rescue[w] {
			t.Errorf("server rescue is missing subcommand %q", w)
		}
	}
}

func TestRunOpRestoresCommandParent(t *testing.T) {
	originalParent := serverUsageCmd.Parent()
	t.Run("invoke", func(t *testing.T) {
		_, _, err := runOp(t, serverUsageCmd, []string{"42"},
			okOp(`{"status":"running","uptime":0,"uptime_text":"0 seconds","cpu":{},"memory":{}}`),
			"table", "", false)
		if err != nil {
			t.Fatalf("RunE: %v", err)
		}
	})

	if got := serverUsageCmd.Parent(); got != originalParent {
		t.Fatalf("command parent = %q, want %q", got.Name(), originalParent.Name())
	}
}

func TestServerOperationCommandsRejectNullResponses(t *testing.T) {
	tests := []struct {
		name  string
		cmd   *cobra.Command
		force bool
	}{
		{name: "rescue enter", cmd: serverRescueEnterCmd, force: true},
		{name: "rescue exit", cmd: serverRescueExitCmd, force: true},
		{name: "boot ISOs", cmd: serverBootISOsCmd},
		{name: "usage", cmd: serverUsageCmd},
		{name: "activity", cmd: serverActivityCmd},
		{name: "retry provision", cmd: serverRetryProvisionCmd},
		{name: "public interface", cmd: serverPublicInterfaceGetCmd},
		{name: "IPv6 reverse DNS", cmd: ipv6ReverseDNSCmd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := runOp(t, tt.cmd, []string{"42"}, okOp(`null`), "table", "", tt.force)
			if err == nil {
				t.Fatal("a null API response must return an error")
			}
		})
	}
}

// --- rescue: {"queued": bool} is the lying-success shape ---

func TestRescueEnterConfirmsBeforeRebooting(t *testing.T) {
	st, _, err := runOp(t, serverRescueEnterCmd, []string{"42"}, okOp(`{"queued":true}`), "table", "n\n", false)
	if err != nil {
		t.Fatalf("declining should not error: %v", err)
	}
	if st.calls != 0 {
		t.Error("declined rescue enter still called the API")
	}
}

func TestRescueEnterSendsISOSlug(t *testing.T) {
	flag := serverRescueEnterCmd.Flags().Lookup("iso")
	originalValue, originalChanged := flag.Value.String(), flag.Changed
	t.Cleanup(func() {
		_ = flag.Value.Set(originalValue)
		flag.Changed = originalChanged
	})
	if err := flag.Value.Set("systemrescue"); err != nil {
		t.Fatalf("set iso flag: %v", err)
	}
	flag.Changed = true

	st, _, err := runOp(t, serverRescueEnterCmd, []string{"42"}, okOp(`{"queued":true}`), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/servers/42/rescue/enter/" {
		t.Errorf("path = %q", st.path)
	}
	if st.body["iso"] != "systemrescue" {
		t.Errorf("iso = %v", st.body["iso"])
	}
}

func TestRescueEnterOmitsISOWhenNotSet(t *testing.T) {
	st, _, err := runOp(t, serverRescueEnterCmd, []string{"42"}, okOp(`{"queued":true}`), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if _, present := st.body["iso"]; present {
		t.Errorf("iso should be omitted so the server picks its default, got %v", st.body)
	}
}

func TestRescueEnterRejectsExplicitEmptyISO(t *testing.T) {
	flag := serverRescueEnterCmd.Flags().Lookup("iso")
	originalValue, originalChanged := flag.Value.String(), flag.Changed
	t.Cleanup(func() {
		_ = flag.Value.Set(originalValue)
		flag.Changed = originalChanged
	})
	if err := flag.Value.Set(""); err != nil {
		t.Fatalf("set iso flag: %v", err)
	}
	flag.Changed = true

	parentOpCommand(t, serverRescueEnterCmd, "table", true)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "")

	err := serverRescueEnterCmd.RunE(serverRescueEnterCmd, []string{"42"})
	if err == nil || err.Error() != "--iso requires a non-empty slug" {
		t.Fatalf("error = %v, want explicit empty --iso rejected", err)
	}
}

func TestRescueEnterFailsWhenNotQueued(t *testing.T) {
	_, _, err := runOp(t, serverRescueEnterCmd, []string{"42"}, okOp(`{"queued":false}`), "table", "", true)
	if err == nil {
		t.Fatal("a 200 with queued=false must not report success")
	}
}

func TestRescueExitFailsWhenNotQueued(t *testing.T) {
	_, _, err := runOp(t, serverRescueExitCmd, []string{"42"}, okOp(`{"queued":false}`), "table", "", true)
	if err == nil {
		t.Fatal("a 200 with queued=false must not report success")
	}
}

func TestRescueExitConfirms(t *testing.T) {
	st, _, err := runOp(t, serverRescueExitCmd, []string{"42"}, okOp(`{"queued":true}`), "table", "n\n", false)
	if err != nil {
		t.Fatalf("declining should not error: %v", err)
	}
	if st.calls != 0 {
		t.Error("declined rescue exit still called the API")
	}
}

// --- retry-provision ---

func TestRetryProvisionFailsWhenNotRetried(t *testing.T) {
	_, _, err := runOp(t, serverRetryProvisionCmd, []string{"42"}, okOp(`{"retry":false}`), "table", "", true)
	if err == nil {
		t.Fatal("a 200 with retry=false must not report success")
	}
}

func TestRetryProvisionReportsSuccess(t *testing.T) {
	st, out, err := runOp(t, serverRetryProvisionCmd, []string{"42"}, okOp(`{"retry":true}`), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/servers/42/retry-provision/" {
		t.Errorf("path = %q", st.path)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("output should name the server: %q", out)
	}
}

// --- boot ISOs: a plain array, and min_ram is a decimal string ---

// The route answers a bare JSON array. Reading it as a paginated envelope
// failed to decode every real response.
func TestBootISOsDecodesPlainArray(t *testing.T) {
	st, out, err := runOp(t, serverBootISOsCmd, []string{"42"}, okOp(
		`[{"slug":"systemrescue","name":"SystemRescue","min_ram":"0.50","compatible":true},`+
			`{"slug":"gparted","name":"GParted","min_ram":"1.00","compatible":false}]`),
		"table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.calls != 1 {
		t.Errorf("calls = %d, want a single request", st.calls)
	}
	if st.path != "/api/cloud/servers/42/boot-isos/" {
		t.Errorf("path = %q, want the per-server catalog", st.path)
	}
	if st.query != "" {
		t.Errorf("query = %q, want no pagination parameters", st.query)
	}
	for _, want := range []string{"systemrescue", "gparted", "0.50", "1.00"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "%!") {
		t.Errorf("min_ram is a decimal string, not a float: %q", out)
	}
}

func TestBootISOsJSONIsTheArray(t *testing.T) {
	_, out, err := runOp(t, serverBootISOsCmd, []string{"42"}, okOp(
		`[{"slug":"systemrescue","name":"SystemRescue","min_ram":"0.50","compatible":true}]`),
		"json", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(got) != 1 || got[0]["slug"] != "systemrescue" || got[0]["min_ram"] != "0.50" {
		t.Errorf("json = %v, want the one ISO with min_ram as sent", got)
	}
}

// --- usage ---

func TestUsageRendersStatusAndUptime(t *testing.T) {
	st, out, err := runOp(t, serverUsageCmd, []string{"42"},
		okOp(`{"status":"running","uptime":3600,"uptime_text":"1 hour","cpu":{"percent":12.5},"memory":{"used":1024}}`),
		"table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/servers/42/usage/" {
		t.Errorf("path = %q", st.path)
	}
	for _, want := range []string{"running", "1 hour"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

// --- activity ---

func TestActivityRendersEntries(t *testing.T) {
	st, out, err := runOp(t, serverActivityCmd, []string{"42"},
		okOp(`{"logs":[{"message":"Server created","date":"2026-08-18T10:00:00Z"},`+
			`{"message":"Package changed","date":"2026-08-18T11:00:00Z"}]}`), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/servers/42/activity/" {
		t.Errorf("path = %q", st.path)
	}
	for _, want := range []string{"Server created", "Package changed"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

// --- public interface ---

const publicIfaceJSON = `{"interface":"eth0","ipv4":"203.0.113.7","ipv6":"2001:db8::1",` +
	`"fw_rules_set":"web","fw_policy_in":"DROP","fw_policy_out":"ACCEPT"}`

func TestPrintPublicInterfaceRendersFirewallName(t *testing.T) {
	pi := pidginhost.NewPublicInterface("eth0", "203.0.113.7", "2001:db8::1")
	pi.SetFwRulesSet("web")
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	if err := printPublicInterface(cmd, pi); err != nil {
		t.Fatalf("printPublicInterface: %v", err)
	}
	if !strings.Contains(out.String(), "web") {
		t.Fatalf("output should contain the firewall set name, got %q", out.String())
	}
}

func TestPrintPublicInterfaceRejectsNilResponse(t *testing.T) {
	if err := printPublicInterface(&cobra.Command{}, nil); err == nil {
		t.Fatal("a null public interface must return an error")
	}
}

func TestPublicInterfaceGetRenders(t *testing.T) {
	st, out, err := runOp(t, serverPublicInterfaceGetCmd, []string{"42"}, okOp(publicIfaceJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodGet || st.path != "/api/cloud/servers/42/public-interface/" {
		t.Errorf("method=%s path=%q", st.method, st.path)
	}
	for _, want := range []string{"eth0", "203.0.113.7", "web", "DROP"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestPublicInterfaceSetSendsOnlyRequestedValues(t *testing.T) {
	serverPublicInterfaceFirewall, serverPublicInterfacePolicyIn = "web", "DROP"
	t.Cleanup(func() { serverPublicInterfaceFirewall, serverPublicInterfacePolicyIn = "", "" })

	st, _, err := runOp(t, serverPublicInterfaceSetCmd, []string{"42"}, okOp(publicIfaceJSON), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodPost {
		t.Errorf("method = %s", st.method)
	}
	if st.body["fw_rules_set"] != "web" {
		t.Errorf("fw_rules_set = %v", st.body["fw_rules_set"])
	}
	if st.body["fw_policy_in"] != "DROP" {
		t.Errorf("fw_policy_in = %v", st.body["fw_policy_in"])
	}
	// The addresses and interface name are read-only; the request model has
	// no place for them, so they must not be sent at all.
	for _, readOnly := range []string{"interface", "ipv4", "ipv6"} {
		if v, present := st.body[readOnly]; present {
			t.Errorf("read-only field %s was sent as %v", readOnly, v)
		}
	}
	if _, present := st.body["fw_policy_out"]; present {
		t.Errorf("unchanged fw_policy_out should be omitted, got %v", st.body["fw_policy_out"])
	}
}

func TestPublicInterfaceSetRejectsUnknownPolicy(t *testing.T) {
	serverPublicInterfacePolicyIn = "MAYBE"
	t.Cleanup(func() { serverPublicInterfacePolicyIn = "" })

	st, _, err := runOp(t, serverPublicInterfaceSetCmd, []string{"42"}, okOp(publicIfaceJSON), "table", "", true)
	if err == nil {
		t.Fatal("expected an error for an invalid policy")
	}
	if st.calls != 0 {
		t.Error("invalid policy should be rejected before calling the API")
	}
}

func TestPublicInterfaceSetRequiresSomethingToChange(t *testing.T) {
	st, _, err := runOp(t, serverPublicInterfaceSetCmd, []string{"42"}, okOp(publicIfaceJSON), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when no field was given to change")
	}
	if st.calls != 0 {
		t.Error("a no-field set should not call the API")
	}
}

func TestPublicInterfaceDeleteConfirms(t *testing.T) {
	st, _, err := runOp(t, serverPublicInterfaceDeleteCmd, []string{"42"}, okOp(``), "table", "n\n", false)
	if err != nil {
		t.Fatalf("declining should not error: %v", err)
	}
	if st.calls != 0 {
		t.Error("declined delete still called the API")
	}
}

func TestPublicInterfaceDeleteRemoves(t *testing.T) {
	st, _, err := runOp(t, serverPublicInterfaceDeleteCmd, []string{"42"},
		func(*opStub) (int, string) { return http.StatusNoContent, `` }, "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodDelete {
		t.Errorf("method = %s", st.method)
	}
}

// --- IPv6 reverse DNS: mirrors the IPv4 command ---

func TestIPv6ReverseDNSReadsWithoutHostname(t *testing.T) {
	st, out, err := runOp(t, ipv6ReverseDNSCmd, []string{"9"},
		okOp(`{"reverse_dns":"host.example.com"}`), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodGet || st.path != "/api/cloud/ipv6/9/rdns/" {
		t.Errorf("method=%s path=%q", st.method, st.path)
	}
	if !strings.Contains(out, "host.example.com") {
		t.Errorf("output = %q", out)
	}
}

func TestIPv6ReverseDNSSetsWithHostname(t *testing.T) {
	if err := ipv6ReverseDNSCmd.Flags().Set("hostname", "new.example.com"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ipv6ReverseDNSCmd.Flags().Set("hostname", "")
		ipv6ReverseDNSCmd.Flags().Lookup("hostname").Changed = false
	})

	st, _, err := runOp(t, ipv6ReverseDNSCmd, []string{"9"},
		okOp(`{"reverse_dns":"new.example.com"}`), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodPost {
		t.Errorf("method = %s", st.method)
	}
	if st.body["reverse_dns"] != "new.example.com" {
		t.Errorf("reverse_dns = %v", st.body["reverse_dns"])
	}
}
