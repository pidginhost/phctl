package kubernetes

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// --- test harness ---
//
// Commands are driven through RunE against a real httptest server reached via
// the package's newClient seam. The seam is restored after every test; callers
// must remain non-parallel because replacing it is a package-global mutation.

type apiCall struct {
	method string
	path   string
	query  url.Values
	body   map[string]any
}

type recorder struct {
	calls []apiCall
}

func (r *recorder) count() int { return len(r.calls) }

func (r *recorder) last() apiCall {
	if len(r.calls) == 0 {
		return apiCall{}
	}
	return r.calls[len(r.calls)-1]
}

// runCmd drives cmd.RunE against a stub API. respond is called once per
// request with the recorded call and its zero-based index, and returns the
// status code and raw body to reply with.
func runCmd(t *testing.T, cmd *cobra.Command, args []string,
	respond func(call apiCall, n int) (int, string),
	format, stdin string, force bool) (*recorder, string, string, error) {
	t.Helper()

	rec := &recorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := apiCall{method: r.Method, path: r.URL.Path, query: r.URL.Query()}
		if r.Body != nil {
			raw, _ := io.ReadAll(r.Body)
			if len(raw) > 0 {
				_ = json.Unmarshal(raw, &call.body)
			}
		}
		n := len(rec.calls)
		rec.calls = append(rec.calls, call)

		code, payload := respond(call, n)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(server.Close)

	oldNewClient := newClient
	newClient = func() (*pidginhost.APIClient, error) {
		return pidginhost.New("test-token", server.URL), nil
	}
	t.Cleanup(func() { newClient = oldNewClient })

	root := &cobra.Command{Use: "phctl"}
	root.PersistentFlags().StringP("output", "o", format, "Output format")
	root.PersistentFlags().BoolP("force", "f", force, "Skip confirmation prompts")
	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)

	var out, errOut bytes.Buffer
	child.SetOut(&out)
	child.SetErr(&errOut)
	child.SetIn(strings.NewReader(stdin))

	err := cmd.RunE(child, args)
	return rec, out.String(), errOut.String(), err
}

// runCmdWithoutAPI drives an input-validation path and reports whether it got
// as far as constructing an API client. Invalid local input must fail before
// config is read or any request can be prepared.
func runCmdWithoutAPI(t *testing.T, cmd *cobra.Command, args []string) (int, error) {
	t.Helper()

	oldNewClient := newClient
	clientCalls := 0
	newClient = func() (*pidginhost.APIClient, error) {
		clientCalls++
		return nil, errors.New("unexpected API client construction")
	}
	defer func() { newClient = oldNewClient }()

	err := cmd.RunE(&cobra.Command{Use: "test"}, args)
	return clientCalls, err
}

// reply answers every request with the same status and payload.
func reply(code int, payload string) func(apiCall, int) (int, string) {
	return func(apiCall, int) (int, string) { return code, payload }
}

// okBody answers every request 200 with payload.
func okBody(payload string) func(apiCall, int) (int, string) {
	return reply(http.StatusOK, payload)
}

// setFlags sets flags on cmd and marks them Changed, then restores the whole
// flag set afterwards so tests do not leak state into one another.
func setFlags(t *testing.T, cmd *cobra.Command, kv map[string]string) {
	t.Helper()
	for name, value := range kv {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("%s has no flag --%s", cmd.Name(), name)
		}
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("setting --%s=%s: %v", name, value, err)
		}
	}
	t.Cleanup(func() {
		for name := range kv {
			resetFlag(cmd, name)
		}
	})
}

// resetFlag returns a flag to its default. Slice flags need Replace: their
// Set appends once the flag has been set, so resetting with Set would leave the
// old entries behind and leak into the next test.
func resetFlag(cmd *cobra.Command, name string) {
	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		return
	}
	if sv, ok := flag.Value.(pflag.SliceValue); ok {
		_ = sv.Replace(nil)
	} else {
		_ = flag.Value.Set(flag.DefValue)
	}
	flag.Changed = false
}

const lbRuleJSON = `{"id":5,"direction":"in","action":"ACCEPT","protocol":"tcp",` +
	`"source":"10.0.0.0/8","sport":null,"destination":null,"dport":"443",` +
	`"comment":"https","enabled":true,"position":1,` +
	`"created":"2026-08-18T10:00:00Z","updated":"2026-08-18T10:00:00Z"}`

// size is an integer and a node's ip is null until it has one.
const poolJSON = `{"id":3,"package":"vm-4","generation":"gen2","size":2,` +
	`"nodes":[{"id":11,"name":"node-a","ip":"10.0.0.11"},{"id":12,"name":"node-b","ip":null}]}`

const nodeJSON = `{"id":11,"name":"node-a","ip":"10.0.0.11"}`

// --- structure ---

func TestKubernetesExposesLBFirewall(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}
	if !names["lb-firewall"] {
		t.Fatal("kubernetes is missing the lb-firewall command")
	}
}

func TestLBFirewallSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range lbFirewallCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get", "create", "update", "delete"} {
		if !names[want] {
			t.Errorf("lb-firewall is missing subcommand %q", want)
		}
	}
}

func TestLBFirewallWriteFlags(t *testing.T) {
	want := []string{"direction", "action", "protocol", "source", "sport",
		"destination", "dport", "comment", "enabled", "position"}
	for _, name := range want {
		if lbFirewallCreateCmd.Flags().Lookup(name) == nil {
			t.Errorf("lb-firewall create missing flag --%s", name)
		}
		if lbFirewallUpdateCmd.Flags().Lookup(name) == nil {
			t.Errorf("lb-firewall update missing flag --%s", name)
		}
	}
}

func TestClusterPhase3Subcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range clusterCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"upgrade-feature", "eligible-vms", "toggle-vm-access"} {
		if !names[want] {
			t.Errorf("cluster is missing subcommand %q", want)
		}
	}
}

func TestUpgradeFeatureFlags(t *testing.T) {
	for _, name := range []string{"feature", "retry"} {
		if clusterUpgradeFeatureCmd.Flags().Lookup(name) == nil {
			t.Errorf("upgrade-feature missing flag --%s", name)
		}
	}
}

func TestPoolPhase3Subcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range poolCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"get", "resize"} {
		if !names[want] {
			t.Errorf("pool is missing subcommand %q", want)
		}
	}
	if poolResizeCmd.Flags().Lookup("size") == nil {
		t.Error("pool resize missing flag --size")
	}
}

func TestNodePhase3Subcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range nodeCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"get", "metrics", "rrd"} {
		if !names[want] {
			t.Errorf("node is missing subcommand %q", want)
		}
	}
}

// --- lb-firewall list ---

func TestLBFirewallListPaginatesAndRendersTable(t *testing.T) {
	rec, out, _, err := runCmd(t, lbFirewallListCmd, []string{"42"}, func(call apiCall, n int) (int, string) {
		if n == 0 {
			return http.StatusOK, `{"count":2,"next":"https://api.test/next","previous":null,"results":[` + lbRuleJSON + `]}`
		}
		second := strings.Replace(lbRuleJSON, `"id":5`, `"id":6`, 1)
		second = strings.Replace(second, `"dport":"443"`, `"dport":"8443"`, 1)
		return http.StatusOK, `{"count":2,"next":null,"previous":null,"results":[` + second + `]}`
	}, "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if rec.count() != 2 {
		t.Fatalf("expected FetchAll to follow the next page, got %d call(s)", rec.count())
	}
	if got := rec.calls[0].path; got != "/api/kubernetes/clusters/42/lb-firewall/" {
		t.Errorf("path = %q", got)
	}
	for _, want := range []string{"ACCEPT", "443", "8443", "10.0.0.0/8", "https"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLBFirewallListRendersJSON(t *testing.T) {
	_, out, _, err := runCmd(t, lbFirewallListCmd, []string{"42"},
		okBody(`{"count":1,"next":null,"previous":null,"results":[`+lbRuleJSON+`]}`), "json", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(got) != 1 || got[0]["dport"] != "443" {
		t.Errorf("unexpected JSON payload: %v", got)
	}
}

func TestLBFirewallListRejectsBadClusterID(t *testing.T) {
	rec, _, _, err := runCmd(t, lbFirewallListCmd, []string{"abc"}, okBody(`{}`), "table", "", false)
	if err == nil {
		t.Fatal("expected an error for a non-numeric cluster ID")
	}
	if rec.count() != 0 {
		t.Errorf("bad cluster ID still reached the API (%d call(s))", rec.count())
	}
}

// --- lb-firewall get ---

func TestLBFirewallGetRendersRule(t *testing.T) {
	rec, out, _, err := runCmd(t, lbFirewallGetCmd, []string{"42", "5"}, okBody(lbRuleJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if got := rec.last().path; got != "/api/kubernetes/clusters/42/lb-firewall/5/" {
		t.Errorf("path = %q", got)
	}
	if !strings.Contains(out, "ACCEPT") {
		t.Errorf("output missing rule action:\n%s", out)
	}
}

func TestLBFirewallGetRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	for _, body := range []string{"", "null"} {
		t.Run("body="+body, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("empty 200 body caused a panic: %v", r)
				}
			}()
			_, _, _, err := runCmd(t, lbFirewallGetCmd, []string{"42", "5"}, okBody(body), "table", "", false)
			if err == nil {
				t.Fatal("expected an error when the server returns no rule")
			}
		})
	}
}

// --- lb-firewall create ---

func TestLBFirewallCreateSendsRule(t *testing.T) {
	setFlags(t, lbFirewallCreateCmd, map[string]string{
		"direction": "in", "action": "ACCEPT", "protocol": "tcp",
		"source": "10.0.0.0/8", "dport": "443", "comment": "https", "position": "1",
	})
	rec, out, _, err := runCmd(t, lbFirewallCreateCmd, []string{"42"},
		reply(http.StatusCreated, lbRuleJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPost || call.path != "/api/kubernetes/clusters/42/lb-firewall/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	for key, want := range map[string]any{
		"direction": "in", "action": "ACCEPT", "protocol": "tcp",
		"source": "10.0.0.0/8", "dport": "443", "comment": "https", "position": float64(1),
	} {
		if call.body[key] != want {
			t.Errorf("body[%s] = %v, want %v", key, call.body[key], want)
		}
	}
	// id, created and updated are assigned by the server and must not be sent.
	for _, readOnly := range []string{"id", "created", "updated"} {
		if v, present := call.body[readOnly]; present {
			t.Errorf("read-only field %s was sent as %v", readOnly, v)
		}
	}
	if !strings.Contains(out, "5") {
		t.Errorf("output missing the new rule ID:\n%s", out)
	}
}

func TestLBFirewallCreateRejectsUnknownEnumBeforeCalling(t *testing.T) {
	for flag, value := range map[string]string{"direction": "sideways", "action": "MAYBE"} {
		t.Run(flag, func(t *testing.T) {
			setFlags(t, lbFirewallCreateCmd, map[string]string{
				"direction": "in", "action": "ACCEPT", flag: value,
			})
			rec, _, _, err := runCmd(t, lbFirewallCreateCmd, []string{"42"},
				reply(http.StatusCreated, lbRuleJSON), "table", "", false)
			if err == nil {
				t.Fatalf("expected an error for --%s=%s", flag, value)
			}
			if rec.count() != 0 {
				t.Errorf("invalid --%s still reached the API (%d call(s))", flag, rec.count())
			}
		})
	}
}

func TestLBFirewallCreateRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	setFlags(t, lbFirewallCreateCmd, map[string]string{"direction": "in", "action": "ACCEPT"})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 201 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, lbFirewallCreateCmd, []string{"42"},
		reply(http.StatusCreated, ""), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the server returns no rule")
	}
}

// The RunE-level tests above cannot prove that Cobra's own flag parsing feeds
// lbRuleFlags.isSet, nor that create and update keep their state apart. Drive
// both through a real command tree.
//
// The tree is built here rather than reusing the package-level Cmd: another
// test in this package reparents Cmd, and executing a command whose parent has
// moved runs the wrong root.
func TestLBFirewallFlagsWorkThroughCobraAndStayIsolated(t *testing.T) {
	rec := &recorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := apiCall{method: r.Method, path: r.URL.Path, query: r.URL.Query()}
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &call.body)
		}
		rec.calls = append(rec.calls, call)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(lbRuleJSON))
			return
		}
		_, _ = w.Write([]byte(strings.Replace(lbRuleJSON, `"dport":"443"`, `"dport":"8443"`, 1)))
	}))
	t.Cleanup(server.Close)

	oldNewClient := newClient
	newClient = func() (*pidginhost.APIClient, error) {
		return pidginhost.New("test-token", server.URL), nil
	}
	t.Cleanup(func() { newClient = oldNewClient })

	originalParent := lbFirewallCmd.Parent()
	root := &cobra.Command{Use: "phctl", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringP("output", "o", "table", "Output format")
	root.PersistentFlags().BoolP("force", "f", true, "Skip confirmation prompts")
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.AddCommand(lbFirewallCmd)
	t.Cleanup(func() {
		if originalParent != nil {
			originalParent.AddCommand(lbFirewallCmd)
		}
		resetFlag(lbFirewallCreateCmd, "source")
		resetFlag(lbFirewallUpdateCmd, "dport")
	})

	root.SetArgs([]string{"lb-firewall", "create", "42", "--source", "10.0.0.0/8"})
	if err := root.Execute(); err != nil {
		t.Fatalf("create through Cobra: %v", err)
	}
	root.SetArgs([]string{"lb-firewall", "update", "42", "5", "--dport", "8443"})
	if err := root.Execute(); err != nil {
		t.Fatalf("update through Cobra: %v", err)
	}

	if rec.count() != 2 {
		t.Fatalf("calls = %d, want 2", rec.count())
	}
	if rec.calls[0].body["source"] != "10.0.0.0/8" {
		t.Errorf("create did not observe its parsed flag: %v", rec.calls[0].body)
	}
	if _, leaked := rec.calls[0].body["dport"]; leaked {
		t.Errorf("update flag leaked into create: %v", rec.calls[0].body)
	}
	if rec.calls[1].body["dport"] != "8443" {
		t.Errorf("update did not observe its parsed flag: %v", rec.calls[1].body)
	}
	if _, leaked := rec.calls[1].body["source"]; leaked {
		t.Errorf("create flag leaked into update: %v", rec.calls[1].body)
	}
}

// --- lb-firewall update ---

func TestLBFirewallUpdateSendsOnlyChangedFields(t *testing.T) {
	setFlags(t, lbFirewallUpdateCmd, map[string]string{"dport": "8443"})
	rec, _, _, err := runCmd(t, lbFirewallUpdateCmd, []string{"42", "5"},
		okBody(strings.Replace(lbRuleJSON, `"dport":"443"`, `"dport":"8443"`, 1)), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPatch {
		t.Errorf("method = %s, want PATCH", call.method)
	}
	if call.path != "/api/kubernetes/clusters/42/lb-firewall/5/" {
		t.Errorf("path = %q", call.path)
	}
	if call.body["dport"] != "8443" {
		t.Errorf("body[dport] = %v", call.body["dport"])
	}
	// A PATCH that carries flags the caller never set would silently overwrite
	// fields with their zero values.
	for _, key := range []string{"direction", "action", "protocol", "source", "sport", "destination", "comment", "enabled", "position"} {
		if _, present := call.body[key]; present {
			t.Errorf("body carries unset field %q: %v", key, call.body)
		}
	}
}

func TestLBFirewallUpdateRequiresAtLeastOneField(t *testing.T) {
	rec, _, _, err := runCmd(t, lbFirewallUpdateCmd, []string{"42", "5"}, okBody(lbRuleJSON), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when no field flags are given")
	}
	if rec.count() != 0 {
		t.Errorf("empty update still reached the API (%d call(s))", rec.count())
	}
}

func TestLBFirewallUpdateRejectsMismatchedRule(t *testing.T) {
	setFlags(t, lbFirewallUpdateCmd, map[string]string{"dport": "8443"})
	_, _, _, err := runCmd(t, lbFirewallUpdateCmd, []string{"42", "5"},
		okBody(strings.Replace(lbRuleJSON, `"id":5`, `"id":9`, 1)), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the server answers with a different rule")
	}
}

func TestLBFirewallUpdateRejectsUnappliedField(t *testing.T) {
	setFlags(t, lbFirewallUpdateCmd, map[string]string{"dport": "8443"})
	_, _, _, err := runCmd(t, lbFirewallUpdateCmd, []string{"42", "5"}, okBody(lbRuleJSON), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the rule still reports the old destination port")
	}
	if !strings.Contains(err.Error(), "dport") {
		t.Errorf("error should identify the unapplied field, got %v", err)
	}
}

func TestLBFirewallUpdateRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	setFlags(t, lbFirewallUpdateCmd, map[string]string{"dport": "8443"})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 200 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, lbFirewallUpdateCmd, []string{"42", "5"}, okBody(""), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the server returns no rule")
	}
}

// --- lb-firewall delete ---

func TestLBFirewallDeleteConfirmsFirst(t *testing.T) {
	rec, _, errOut, err := runCmd(t, lbFirewallDeleteCmd, []string{"42", "5"},
		reply(http.StatusNoContent, ""), "table", "n\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if rec.count() != 0 {
		t.Errorf("declined delete still reached the API (%d call(s))", rec.count())
	}
	if !strings.Contains(errOut, "[y/N]") {
		t.Errorf("no confirmation prompt on stderr: %q", errOut)
	}
}

func TestLBFirewallDeleteWithForce(t *testing.T) {
	rec, _, _, err := runCmd(t, lbFirewallDeleteCmd, []string{"42", "5"},
		reply(http.StatusNoContent, ""), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodDelete || call.path != "/api/kubernetes/clusters/42/lb-firewall/5/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
}

// --- cluster upgrade-feature ---

func TestUpgradeFeatureSendsRequestAndConfirms(t *testing.T) {
	setFlags(t, clusterUpgradeFeatureCmd, map[string]string{"feature": "cert-manager", "retry": "true"})
	rec, out, _, err := runCmd(t, clusterUpgradeFeatureCmd, []string{"42"},
		okBody(`{"status":"OK","message":"Upgrading cert-manager to 1.16..."}`), "table", "y\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPost || call.path != "/api/kubernetes/clusters/42/upgrade-feature/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if call.body["feature_name"] != "cert-manager" || call.body["retry"] != true {
		t.Errorf("body = %v", call.body)
	}
	if !strings.Contains(out, "cert-manager") {
		t.Errorf("output missing the server message:\n%s", out)
	}
}

func TestUpgradeFeatureDeclinedDoesNotCall(t *testing.T) {
	setFlags(t, clusterUpgradeFeatureCmd, map[string]string{"feature": "cert-manager"})
	rec, _, _, err := runCmd(t, clusterUpgradeFeatureCmd, []string{"42"},
		okBody(`{"status":"OK","message":"go"}`), "table", "n\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if rec.count() != 0 {
		t.Errorf("declined upgrade still reached the API (%d call(s))", rec.count())
	}
}

func TestUpgradeFeatureRequiresFeatureName(t *testing.T) {
	rec, _, _, err := runCmd(t, clusterUpgradeFeatureCmd, []string{"42"},
		okBody(`{"status":"OK","message":"go"}`), "table", "y\n", true)
	if err == nil {
		t.Fatal("expected an error when --feature is missing")
	}
	if rec.count() != 0 {
		t.Errorf("missing --feature still reached the API (%d call(s))", rec.count())
	}
}

func TestUpgradeFeatureRejectsNonOKStatus(t *testing.T) {
	setFlags(t, clusterUpgradeFeatureCmd, map[string]string{"feature": "cert-manager"})
	_, _, _, err := runCmd(t, clusterUpgradeFeatureCmd, []string{"42"},
		okBody(`{"status":"SKIPPED","message":"nothing to do"}`), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the server reports a non-OK status")
	}
	if !strings.Contains(err.Error(), "SKIPPED") {
		t.Errorf("error should quote the reported status, got %v", err)
	}
}

func TestUpgradeFeatureRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	setFlags(t, clusterUpgradeFeatureCmd, map[string]string{"feature": "cert-manager"})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 200 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, clusterUpgradeFeatureCmd, []string{"42"}, okBody(""), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the server returns no result")
	}
}

// --- cluster eligible-vms ---

func TestEligibleVMsRendersTable(t *testing.T) {
	rec, out, _, err := runCmd(t, clusterEligibleVMsCmd, []string{"42"},
		okBody(`{"vms":[{"id":7,"hostname":"web-1"},{"id":8,"hostname":"web-2"}]}`), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if got := rec.last().path; got != "/api/kubernetes/clusters/42/eligible-vms/" {
		t.Errorf("path = %q", got)
	}
	for _, want := range []string{"web-1", "web-2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestEligibleVMsRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 200 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, clusterEligibleVMsCmd, []string{"42"}, okBody(""), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the server returns no result")
	}
}

// --- cluster toggle-vm-access ---

func TestToggleVMAccessConfirmsAndReportsNewState(t *testing.T) {
	rec, out, _, err := runCmd(t, clusterToggleVMAccessCmd, []string{"42"},
		okBody(`{"enabled":true,"message":"Cloud VM access enabled."}`), "table", "y\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPost || call.path != "/api/kubernetes/clusters/42/toggle-cloud-vm-access/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if !strings.Contains(out, "enabled") {
		t.Errorf("output missing the new state:\n%s", out)
	}
}

func TestToggleVMAccessDeclinedDoesNotCall(t *testing.T) {
	rec, _, _, err := runCmd(t, clusterToggleVMAccessCmd, []string{"42"},
		okBody(`{"enabled":true,"message":"x"}`), "table", "n\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if rec.count() != 0 {
		t.Errorf("declined toggle still reached the API (%d call(s))", rec.count())
	}
}

func TestToggleVMAccessRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 200 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, clusterToggleVMAccessCmd, []string{"42"}, okBody(""), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the server returns no result")
	}
}

// --- pool get ---

func TestPoolGetRendersPool(t *testing.T) {
	rec, out, _, err := runCmd(t, poolGetCmd, []string{"42", "3"}, okBody(poolJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if got := rec.last().path; got != "/api/kubernetes/clusters/42/resource-pools/3/" {
		t.Errorf("path = %q", got)
	}
	for _, want := range []string{"vm-4", "node-a", "10.0.0.11"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// A node without an address renders as <none>, not as the Go struct
	// behind the nullable field.
	lines := strings.Split(out, "\n")
	var nodeB string
	for _, line := range lines {
		if strings.Contains(line, "node-b") {
			nodeB = line
		}
	}
	if got := strings.Fields(nodeB); len(got) != 3 || got[2] != "<none>" {
		t.Errorf("node-b row = %q, want its IP as <none>", nodeB)
	}
	if !strings.Contains(out, "Size:") || !strings.Contains(out, " 2\n") {
		t.Errorf("output missing the pool size 2:\n%s", out)
	}
}

func TestPoolGetRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 200 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, poolGetCmd, []string{"42", "3"}, okBody(""), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the server returns no pool")
	}
}

// --- pool resize ---

func TestPoolResizeConfirmsAndPatchesNewSize(t *testing.T) {
	setFlags(t, poolResizeCmd, map[string]string{"size": "4"})
	rec, out, errOut, err := runCmd(t, poolResizeCmd, []string{"42", "3"}, okBody(poolJSON), "table", "y\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPatch || call.path != "/api/kubernetes/clusters/42/resource-pools/3/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if call.body["new_size"] != float64(4) {
		t.Errorf("body = %v", call.body)
	}
	// size is an integer; a string verb would print %!s(int32=2).
	if want := "Resize of pool 3 to 4 node(s) requested; it currently reports 2.\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
	if !strings.Contains(errOut, "[y/N]") {
		t.Errorf("resize did not confirm: %q", errOut)
	}
}

func TestPoolResizeDeclinedDoesNotCall(t *testing.T) {
	setFlags(t, poolResizeCmd, map[string]string{"size": "4"})
	rec, _, _, err := runCmd(t, poolResizeCmd, []string{"42", "3"}, okBody(poolJSON), "table", "n\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if rec.count() != 0 {
		t.Errorf("declined resize still reached the API (%d call(s))", rec.count())
	}
}

func TestPoolResizeRequiresPositiveSize(t *testing.T) {
	for _, size := range []string{"0", "-1"} {
		t.Run("size="+size, func(t *testing.T) {
			setFlags(t, poolResizeCmd, map[string]string{"size": size})
			rec, _, _, err := runCmd(t, poolResizeCmd, []string{"42", "3"}, okBody(poolJSON), "table", "y\n", true)
			if err == nil {
				t.Fatalf("expected an error for --size=%s", size)
			}
			if rec.count() != 0 {
				t.Errorf("--size=%s still reached the API (%d call(s))", size, rec.count())
			}
		})
	}
}

func TestPoolResizeRejectsMismatchedPool(t *testing.T) {
	setFlags(t, poolResizeCmd, map[string]string{"size": "4"})
	_, _, _, err := runCmd(t, poolResizeCmd, []string{"42", "3"},
		okBody(strings.Replace(poolJSON, `"id":3`, `"id":9`, 1)), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the server answers with a different pool")
	}
}

func TestPoolResizeRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	setFlags(t, poolResizeCmd, map[string]string{"size": "4"})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 200 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, poolResizeCmd, []string{"42", "3"}, okBody(""), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the server returns no pool")
	}
}

// --- node get ---

func TestNodeGetRendersNode(t *testing.T) {
	rec, out, _, err := runCmd(t, nodeGetCmd, []string{"42", "3", "11"}, okBody(nodeJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if got := rec.last().path; got != "/api/kubernetes/clusters/42/resource-pools/3/nodes/11/" {
		t.Errorf("path = %q", got)
	}
	if !strings.Contains(out, "node-a") {
		t.Errorf("output missing the node name:\n%s", out)
	}
	if !strings.Contains(out, "10.0.0.11") {
		t.Errorf("output missing the node IP:\n%s", out)
	}
}

func TestNodeGetRendersMissingIPAsNone(t *testing.T) {
	_, out, _, err := runCmd(t, nodeGetCmd, []string{"42", "3", "11"},
		okBody(`{"id":11,"name":"node-a","ip":null}`), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !strings.Contains(out, "IP:    <none>") {
		t.Errorf("output = %q, want the IP as <none>", out)
	}
}

func TestNodeListRendersIP(t *testing.T) {
	_, out, _, err := runCmd(t, nodeListCmd, []string{"42", "3"},
		okBody(`{"count":2,"next":null,"previous":null,"results":[`+
			`{"id":11,"name":"node-a","ip":"10.0.0.11"},{"id":12,"name":"node-b","ip":null}]}`),
		"table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	for _, want := range []string{"10.0.0.11", "<none>"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "{") {
		t.Errorf("a nullable field leaked its Go struct into the table:\n%s", out)
	}
}

// --- node delete: the API starts an operation and answers 202 with it ---

const nodeOperationJSON = `{"id":77,"kind":"delete","source":"api","target_hostname":"node-a",` +
	`"status":"pending","reason":"","message":"","bypass_pdb":false,"delete_unmanaged_pods":false,` +
	`"local_data_loss_accepted":false,"bypass_pdb_confirmed_at":null,"unmanaged_pods_confirmed_at":null,` +
	`"actor_label":"user@example.com","created_at":"2026-10-01T10:00:00Z","updated_at":"2026-10-01T10:00:00Z",` +
	`"finished_at":null,"allowed_actions":["cancel"]}`

func TestNodeDeleteReportsTheOperation(t *testing.T) {
	rec, out, errOut, err := runCmd(t, nodeDeleteCmd, []string{"42", "3", "11"},
		reply(http.StatusAccepted, nodeOperationJSON), "table", "y\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodDelete || call.path != "/api/kubernetes/clusters/42/resource-pools/3/nodes/11/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if !strings.Contains(errOut, "[y/N]") {
		t.Errorf("delete did not confirm: %q", errOut)
	}
	// The node is not gone yet: the route only admits the removal.
	if strings.Contains(out, "deleted") {
		t.Errorf("output claims the node is already deleted: %q", out)
	}
	for _, want := range []string{"11", "77", "pending"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestNodeDeleteJSONIsTheOperation(t *testing.T) {
	_, out, _, err := runCmd(t, nodeDeleteCmd, []string{"42", "3", "11"},
		reply(http.StatusAccepted, nodeOperationJSON), "json", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if got["id"] != float64(77) || got["status"] != "pending" {
		t.Errorf("json = %v, want the operation", got)
	}
}

func TestNodeDeleteRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 202 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, nodeDeleteCmd, []string{"42", "3", "11"}, reply(http.StatusAccepted, ""), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the server returns no operation")
	}
}

func TestNodeGetRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 200 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, nodeGetCmd, []string{"42", "3", "11"}, okBody(""), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the server returns no node")
	}
}

// --- node rrd ---

func TestNodeRRDRendersDataPoints(t *testing.T) {
	rec, out, _, err := runCmd(t, nodeRRDCmd, []string{"42", "3", "11"},
		okBody(`{"timeframe":"hour","data":[{"time":1755500000,"cpu":0.1,"mem":100,"maxmem":200,"netin":1,"netout":2,"diskread":3,"diskwrite":4}]}`),
		"table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if got := rec.last().path; got != "/api/kubernetes/clusters/42/resource-pools/3/nodes/11/rrd/" {
		t.Errorf("path = %q", got)
	}
	if !strings.Contains(out, "hour") {
		t.Errorf("output missing the timeframe:\n%s", out)
	}
	for _, want := range []string{"1755500000", "DISKREAD", "DISKWRITE", "3", "4"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "1.7555e+09") {
		t.Errorf("timestamp should not use scientific notation:\n%s", out)
	}
}

func TestNodeRRDSurvivesUntypedDataPoints(t *testing.T) {
	// The schema types "data" as an untyped array, so anything can arrive.
	_, _, _, err := runCmd(t, nodeRRDCmd, []string{"42", "3", "11"},
		okBody(`{"timeframe":"hour","data":["unexpected",42,null]}`), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
}

func TestNodeRRDRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 200 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, nodeRRDCmd, []string{"42", "3", "11"}, okBody(""), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the server returns no data")
	}
}

// --- cluster update ---

// The wire shape the API really sends. price_per_month is a DecimalField and
// stays a string; price_per_hour is derived and arrives as a number, and the
// four fields after it are a bool or an integer. The schema used to declare all
// five as strings, which is why this fixture was wrong and the command could
// not have decoded a real cluster.
const clusterDetailJSON = `{"id":42,"status":"active","name":"prod","generation":"gen2",` +
	`"cluster_type":"ha","kube_version":"1.32","price_per_month":"120.5000","price_per_hour":0.165,` +
	`"features":["cert-manager"],"features_ready":true,"kubeconfig_valid_until":"2027-01-01T00:00:00Z",` +
	`"ipv4_address":"203.0.113.10","ipv6_address":"","dual_stack":false,"protected":true,` +
	`"talos_version":"1.9.0","talos_upgrade_available":false,"talos_next_version":"",` +
	`"storage_quota_gb":100,"last_pool_used_bytes":0,"last_storage_sync_at":""}`

func TestClusterUpdateIsRegistered(t *testing.T) {
	names := map[string]bool{}
	for _, c := range clusterCmd.Commands() {
		names[c.Name()] = true
	}
	if !names["update"] {
		t.Fatal("cluster is missing the update subcommand")
	}
	for _, name := range []string{"name", "protected", "features"} {
		if clusterUpdateCmd.Flags().Lookup(name) == nil {
			t.Errorf("cluster update missing flag --%s", name)
		}
	}
}

func TestClusterUpdateRequiresAtLeastOneField(t *testing.T) {
	rec, _, _, err := runCmd(t, clusterUpdateCmd, []string{"42"}, okBody(clusterDetailJSON), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when no field flags are given")
	}
	if rec.count() != 0 {
		t.Errorf("empty update still reached the API (%d call(s))", rec.count())
	}
}

func TestClusterUpdateSendsOnlyChangedFields(t *testing.T) {
	setFlags(t, clusterUpdateCmd, map[string]string{"name": "prod"})
	rec, out, _, err := runCmd(t, clusterUpdateCmd, []string{"42"}, okBody(clusterDetailJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPatch || call.path != "/api/kubernetes/clusters/42/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if call.body["name"] != "prod" {
		t.Errorf("body[name] = %v", call.body["name"])
	}
	for _, key := range []string{"protected", "features", "features_ready"} {
		if _, present := call.body[key]; present {
			t.Errorf("body carries unset field %q: %v", key, call.body)
		}
	}
	if !strings.Contains(out, "42") {
		t.Errorf("output missing the cluster ID:\n%s", out)
	}
}

// A rename that the server does not apply must not be reported as success.
func TestClusterUpdateRejectsUnappliedName(t *testing.T) {
	setFlags(t, clusterUpdateCmd, map[string]string{"name": "staging"})
	_, _, _, err := runCmd(t, clusterUpdateCmd, []string{"42"}, okBody(clusterDetailJSON), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the cluster still reports the old name")
	}
	if !strings.Contains(err.Error(), "staging") {
		t.Errorf("error should name what was asked for, got %v", err)
	}
}

func TestClusterUpdateRejectsUnappliedProtected(t *testing.T) {
	setFlags(t, clusterUpdateCmd, map[string]string{"protected": "false"})
	_, _, _, err := runCmd(t, clusterUpdateCmd, []string{"42"}, okBody(clusterDetailJSON), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the cluster still reports protected=true")
	}
}

func TestClusterUpdateConfirmsFeatureChanges(t *testing.T) {
	setFlags(t, clusterUpdateCmd, map[string]string{"features": "cert-manager"})
	rec, _, errOut, err := runCmd(t, clusterUpdateCmd, []string{"42"}, okBody(clusterDetailJSON), "table", "n\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if rec.count() != 0 {
		t.Errorf("declined feature change still reached the API (%d call(s))", rec.count())
	}
	if !strings.Contains(errOut, "[y/N]") {
		t.Errorf("feature change did not confirm: %q", errOut)
	}
}

func TestClusterUpdateSendsFeatures(t *testing.T) {
	setFlags(t, clusterUpdateCmd, map[string]string{"features": "cert-manager"})
	rec, _, _, err := runCmd(t, clusterUpdateCmd, []string{"42"}, okBody(clusterDetailJSON), "table", "y\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	features, _ := rec.last().body["features"].([]any)
	if len(features) != 1 || features[0] != "cert-manager" {
		t.Errorf("body[features] = %v", rec.last().body["features"])
	}
}

func TestClusterUpdateRejectsUnknownFeature(t *testing.T) {
	setFlags(t, clusterUpdateCmd, map[string]string{"features": "cert-manager,teapot"})
	rec, _, _, err := runCmd(t, clusterUpdateCmd, []string{"42"}, okBody(clusterDetailJSON), "table", "y\n", true)
	if err == nil {
		t.Fatal("expected an error for an unknown feature")
	}
	if rec.count() != 0 {
		t.Errorf("unknown feature still reached the API (%d call(s))", rec.count())
	}
}

func TestClusterUpdateRejectsDuplicateFeatures(t *testing.T) {
	setFlags(t, clusterUpdateCmd, map[string]string{"features": "cert-manager,cert-manager"})
	rec, _, _, err := runCmd(t, clusterUpdateCmd, []string{"42"}, okBody(clusterDetailJSON), "table", "", true)
	if err == nil {
		t.Fatal("expected an error for a duplicate feature in a feature set")
	}
	if rec.count() != 0 {
		t.Errorf("duplicate feature still reached the API (%d call(s))", rec.count())
	}
}

// The generated model drops an empty feature list (omitempty), so a request to
// remove every feature would leave the cluster untouched while answering 200.
func TestClusterUpdateRejectsSilentlyDroppedFeatureClear(t *testing.T) {
	setFlags(t, clusterUpdateCmd, map[string]string{"features": ""})
	_, _, _, err := runCmd(t, clusterUpdateCmd, []string{"42"}, okBody(clusterDetailJSON), "table", "y\n", true)
	if err == nil {
		t.Fatal("expected an error: clearing every feature cannot be expressed and must not look like success")
	}
}

func TestClusterUpdateRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	setFlags(t, clusterUpdateCmd, map[string]string{"name": "prod"})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 200 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, clusterUpdateCmd, []string{"42"}, okBody(""), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the server returns no cluster")
	}
}

func TestClusterUpdateRejectsMismatchedCluster(t *testing.T) {
	setFlags(t, clusterUpdateCmd, map[string]string{"name": "prod"})
	_, _, _, err := runCmd(t, clusterUpdateCmd, []string{"42"},
		okBody(strings.Replace(clusterDetailJSON, `"id":42`, `"id":9`, 1)), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the server answers with a different cluster")
	}
}

// --- kubeconfig regenerate ---

func TestKubeconfigRegenerateHasFlag(t *testing.T) {
	if clusterKubeconfigCmd.Flags().Lookup("regenerate") == nil {
		t.Fatal("cluster kubeconfig missing --regenerate flag")
	}
}

func TestKubeconfigDefaultsToGet(t *testing.T) {
	rec, out, _, err := runCmd(t, clusterKubeconfigCmd, []string{"42"},
		okBody("apiVersion: v1\n"), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodGet || call.path != "/api/kubernetes/clusters/42/kubeconfig/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if !strings.Contains(out, "apiVersion: v1") {
		t.Errorf("output missing the kubeconfig:\n%s", out)
	}
}

func TestKubeconfigRegenerateConfirmsAndPosts(t *testing.T) {
	setFlags(t, clusterKubeconfigCmd, map[string]string{"regenerate": "true"})
	rec, out, errOut, err := runCmd(t, clusterKubeconfigCmd, []string{"42"},
		okBody("apiVersion: v1\nclusters: []\n"), "table", "y\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPost || call.path != "/api/kubernetes/clusters/42/kubeconfig/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if !strings.Contains(errOut, "[y/N]") {
		t.Errorf("regenerate did not confirm: %q", errOut)
	}
	if !strings.Contains(out, "apiVersion: v1") {
		t.Errorf("output missing the new kubeconfig:\n%s", out)
	}
}

func TestKubeconfigRegenerateDeclinedDoesNotCall(t *testing.T) {
	setFlags(t, clusterKubeconfigCmd, map[string]string{"regenerate": "true"})
	rec, _, _, err := runCmd(t, clusterKubeconfigCmd, []string{"42"},
		okBody("apiVersion: v1\n"), "table", "n\n", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if rec.count() != 0 {
		t.Errorf("declined regenerate still reached the API (%d call(s))", rec.count())
	}
}

func TestKubeconfigRejectsEmptyBody(t *testing.T) {
	_, _, _, err := runCmd(t, clusterKubeconfigCmd, []string{"42"}, okBody(""), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the server returns an empty kubeconfig")
	}
}

func TestKubeconfigRejectsBlankBody(t *testing.T) {
	_, _, _, err := runCmd(t, clusterKubeconfigCmd, []string{"42"}, okBody(" \n\t"), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the server returns a whitespace-only kubeconfig")
	}
}

// update sends only the flags you pass, so advertising a default for one would
// read as "omit this and it resets to in/ACCEPT". create's defaults are real:
// they are what the rule ends up with.
func TestLBFirewallUpdateAdvertisesNoDefaults(t *testing.T) {
	for _, name := range lbRuleFieldFlags {
		flag := lbFirewallUpdateCmd.Flags().Lookup(name)
		if flag == nil {
			t.Fatalf("lb-firewall update missing flag --%s", name)
		}
		switch flag.DefValue {
		case "", "false", "0":
		default:
			t.Errorf("lb-firewall update --%s advertises default %q", name, flag.DefValue)
		}
	}
	if got := lbFirewallCreateCmd.Flags().Lookup("direction").DefValue; got != "in" {
		t.Errorf("lb-firewall create --direction default = %q, want in", got)
	}
	if got := lbFirewallCreateCmd.Flags().Lookup("action").DefValue; got != "ACCEPT" {
		t.Errorf("lb-firewall create --action default = %q, want ACCEPT", got)
	}
}
