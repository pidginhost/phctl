package storage

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const (
	testAccessKey = "AKIAEXAMPLEACCESSKEY"
	testSecretKey = "s3cr3tEXAMPLEsecretkeyvalue"
)

const bucketJSON = `{"id":7,"name":"assets","full_name":"c1-assets","quota_gb":50,` +
	`"used_bytes":1048576,"object_count":12,"public_read":false,"status":"active",` +
	`"endpoint":"https://s3.example","region":"eu-1","created":"2026-08-18T10:00:00Z"}`

type stub struct {
	method string
	path   string
	body   map[string]interface{}
	called bool
}

// run drives a command's RunE against a stub API. stdin feeds confirmation
// prompts; format sets the global -o/--output; force sets the global -f.
func run(t *testing.T, cmd *cobra.Command, args []string, respond func(*stub) (int, string),
	format, stdin string, force bool) (*stub, string, string, error) {
	t.Helper()

	st := &stub{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st.called = true
		st.method = r.Method
		st.path = r.URL.Path
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
	return st, out.String(), errOut.String(), err
}

func ok(payload string) func(*stub) (int, string) {
	return func(*stub) (int, string) { return http.StatusOK, payload }
}

// --- structure ---

func TestStorageGroupExposesBucket(t *testing.T) {
	if Cmd.Use != "storage" {
		t.Errorf("Use = %q, want storage", Cmd.Use)
	}
	var found bool
	for _, c := range Cmd.Commands() {
		if c.Name() == "bucket" {
			found = true
		}
	}
	if !found {
		t.Fatal("storage group is missing the bucket command")
	}
}

func TestBucketSubcommands(t *testing.T) {
	want := []string{"list", "get", "create", "delete", "resize", "visibility", "credentials"}
	have := map[string]bool{}
	for _, c := range bucketCmd.Commands() {
		have[c.Name()] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("bucket is missing subcommand %q", w)
		}
	}
	creds := map[string]bool{}
	for _, c := range bucketCredentialsCmd.Commands() {
		creds[c.Name()] = true
	}
	for _, w := range []string{"reveal", "rotate"} {
		if !creds[w] {
			t.Errorf("bucket credentials is missing subcommand %q", w)
		}
	}
}

// --- read ---

func TestBucketListRendersTable(t *testing.T) {
	st, out, _, err := run(t, bucketListCmd, nil, ok("["+bucketJSON+"]"), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/buckets/" {
		t.Errorf("path = %q", st.path)
	}
	for _, want := range []string{"assets", "c1-assets", "50", "eu-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestBucketGetRendersJSON(t *testing.T) {
	_, out, _, err := run(t, bucketGetCmd, []string{"7"}, ok(bucketJSON), "json", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if got["full_name"] != "c1-assets" {
		t.Errorf("full_name = %v", got["full_name"])
	}
}

// --- create (billable: confirms) ---

func TestBucketCreateSendsWritableFields(t *testing.T) {
	bucketCreateName, bucketCreateQuota, bucketCreatePublic = "assets", 50, true
	t.Cleanup(func() { bucketCreateName, bucketCreateQuota, bucketCreatePublic = "", 0, false })

	st, _, _, err := run(t, bucketCreateCmd, nil,
		func(*stub) (int, string) { return http.StatusAccepted, bucketJSON }, "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodPost {
		t.Errorf("method = %s", st.method)
	}
	if st.body["name"] != "assets" {
		t.Errorf("name = %v", st.body["name"])
	}
	if st.body["quota_gb"] != float64(50) {
		t.Errorf("quota_gb = %v", st.body["quota_gb"])
	}
	if st.body["public_read"] != true {
		t.Errorf("public_read = %v", st.body["public_read"])
	}
}

func TestBucketCreateAbortsWithoutConfirmation(t *testing.T) {
	bucketCreateName, bucketCreateQuota = "assets", 50
	t.Cleanup(func() { bucketCreateName, bucketCreateQuota = "", 0 })

	st, _, _, err := run(t, bucketCreateCmd, nil, ok(bucketJSON), "table", "n\n", false)
	if err != nil {
		t.Fatalf("declining should not error: %v", err)
	}
	if st.called {
		t.Error("declined create still called the API")
	}
}

// --- delete (destructive: confirms) ---

func TestBucketDeleteAbortsWithoutConfirmation(t *testing.T) {
	st, _, _, err := run(t, bucketDeleteCmd, []string{"7"},
		ok(`{"id":7,"status":"cancelling"}`), "table", "n\n", false)
	if err != nil {
		t.Fatalf("declining should not error: %v", err)
	}
	if st.called {
		t.Error("declined delete still called the API")
	}
}

func TestBucketDeleteReportsCancelling(t *testing.T) {
	st, out, _, err := run(t, bucketDeleteCmd, []string{"7"},
		func(*stub) (int, string) { return http.StatusAccepted, `{"id":7,"status":"cancelling"}` },
		"table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodDelete {
		t.Errorf("method = %s", st.method)
	}
	if !strings.Contains(out, "cancelling") {
		t.Errorf("output should report the status it got: %q", out)
	}
}

// --- resize ---

func TestBucketResizeSendsQuota(t *testing.T) {
	bucketResizeQuota = 100
	t.Cleanup(func() { bucketResizeQuota = 0 })

	resized := strings.Replace(bucketJSON, `"quota_gb":50`, `"quota_gb":100`, 1)
	st, _, _, err := run(t, bucketResizeCmd, []string{"7"}, ok(resized), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/buckets/7/resize/" {
		t.Errorf("path = %q", st.path)
	}
	if st.body["quota_gb"] != float64(100) {
		t.Errorf("quota_gb = %v", st.body["quota_gb"])
	}
}

// A 200 that comes back still at the old quota means the resize did not happen.
// Reporting success there tells an operator something untrue.
func TestBucketResizeFailsWhenQuotaDidNotChange(t *testing.T) {
	bucketResizeQuota = 100
	t.Cleanup(func() { bucketResizeQuota = 0 })

	_, _, _, err := run(t, bucketResizeCmd, []string{"7"}, ok(bucketJSON), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the response still reports the old quota")
	}
	if !strings.Contains(err.Error(), "50") {
		t.Errorf("error should say what the quota actually is: %v", err)
	}
}

// --- visibility ---

func TestBucketVisibilityPublicSendsTrue(t *testing.T) {
	bucketVisibilityPublic = true
	t.Cleanup(func() { bucketVisibilityPublic = false })

	pub := strings.Replace(bucketJSON, `"public_read":false`, `"public_read":true`, 1)
	st, _, _, err := run(t, bucketVisibilityCmd, []string{"7"}, ok(pub), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/buckets/7/visibility/" {
		t.Errorf("path = %q", st.path)
	}
	if st.body["public_read"] != true {
		t.Errorf("public_read = %v", st.body["public_read"])
	}
}

func TestBucketVisibilityGoingPublicConfirms(t *testing.T) {
	bucketVisibilityPublic = true
	t.Cleanup(func() { bucketVisibilityPublic = false })

	st, _, _, err := run(t, bucketVisibilityCmd, []string{"7"}, ok(bucketJSON), "table", "n\n", false)
	if err != nil {
		t.Fatalf("declining should not error: %v", err)
	}
	if st.called {
		t.Error("declined visibility change still called the API")
	}
}

func TestBucketVisibilityFailsWhenFlagDidNotChange(t *testing.T) {
	bucketVisibilityPublic = true
	t.Cleanup(func() { bucketVisibilityPublic = false })

	_, _, _, err := run(t, bucketVisibilityCmd, []string{"7"}, ok(bucketJSON), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the response still reports the old visibility")
	}
}

// --- credentials ---

const credsJSON = `{"bucket":"c1-assets","endpoint":"https://s3.example","region":"eu-1",` +
	`"access_key":"` + testAccessKey + `","secret_key":"` + testSecretKey + `"}`

func TestCredentialsRevealRequiresConfirmation(t *testing.T) {
	st, out, _, err := run(t, bucketCredentialsRevealCmd, []string{"7"}, ok(credsJSON), "table", "n\n", false)
	if err != nil {
		t.Fatalf("declining should not error: %v", err)
	}
	if st.called {
		t.Error("declined reveal still called the API")
	}
	if strings.Contains(out, testSecretKey) {
		t.Error("declined reveal printed the secret anyway")
	}
}

func TestCredentialsRevealWithForcePrintsKeys(t *testing.T) {
	st, out, _, err := run(t, bucketCredentialsRevealCmd, []string{"7"}, ok(credsJSON), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.path != "/api/cloud/buckets/7/credentials/reveal/" {
		t.Errorf("path = %q", st.path)
	}
	for _, want := range []string{testAccessKey, testSecretKey, "c1-assets"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestCredentialsRotateConfirms(t *testing.T) {
	st, _, _, err := run(t, bucketCredentialsRotateCmd, []string{"7"}, ok(credsJSON), "table", "n\n", false)
	if err != nil {
		t.Fatalf("declining should not error: %v", err)
	}
	if st.called {
		t.Error("declined rotate still called the API -- the old keys would be dead")
	}
}

func TestCredentialsRotatePrintsNewKeys(t *testing.T) {
	_, out, _, err := run(t, bucketCredentialsRotateCmd, []string{"7"}, ok(credsJSON), "json", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if got["secret_key"] != testSecretKey {
		t.Errorf("secret_key = %v", got["secret_key"])
	}
}

// The reason APIErrorRedacted exists: a 200 the model cannot decode must not
// put a live secret into an error, where it reaches logs and bug reports.
func TestCredentialsErrorsNeverCarryTheSecret(t *testing.T) {
	undecodable := `{"bucket":"c1-assets","endpoint":"https://s3.example","region":"eu-1",` +
		`"access_key":12345,"secret_key":"` + testSecretKey + `"}`

	for name, cmd := range map[string]*cobra.Command{
		"reveal": bucketCredentialsRevealCmd,
		"rotate": bucketCredentialsRotateCmd,
	} {
		t.Run(name, func(t *testing.T) {
			_, out, errOut, err := run(t, cmd, []string{"7"}, ok(undecodable), "table", "", true)
			if err == nil {
				t.Fatal("expected a decode error")
			}
			for _, sink := range []string{err.Error(), out, errOut} {
				if strings.Contains(sink, testSecretKey) {
					t.Errorf("secret leaked: %s", sink)
				}
			}
		})
	}
}
