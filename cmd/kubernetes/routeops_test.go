package kubernetes

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const httpRouteJSON = `{"id":4,"name":"web","namespace":"default","hostnames":["example.com"],` +
	`"backend_service_name":"web-svc","backend_service_port":8080,"backend_namespace":"default",` +
	`"path_prefix":"/","enable_tls":true,"status_ready":true,"status_message":"Programmed",` +
	`"created":"2026-08-18T10:00:00Z","updated":"2026-08-18T10:00:00Z"}`

const tcpRouteJSON = `{"id":6,"name":"pg","namespace":"default","port":5432,` +
	`"backend_service_name":"pg-svc","backend_service_port":5432,"backend_namespace":"default",` +
	`"status_ready":true,"status_message":"Programmed",` +
	`"created":"2026-08-18T10:00:00Z","updated":"2026-08-18T10:00:00Z"}`

const udpRouteJSON = `{"id":7,"name":"dns","namespace":"default","port":5353,` +
	`"backend_service_name":"dns-svc","backend_service_port":53,"backend_namespace":"default",` +
	`"status_ready":false,"status_message":"Pending",` +
	`"created":"2026-08-18T10:00:00Z","updated":"2026-08-18T10:00:00Z"}`

// --- structure ---

func TestRouteGroupsGainGetAndUpdate(t *testing.T) {
	for name, group := range map[string]*cobra.Command{
		"http-route": httpRouteCmd,
		"tcp-route":  tcpRouteCmd,
		"udp-route":  udpRouteCmd,
	} {
		names := map[string]bool{}
		for _, c := range group.Commands() {
			names[c.Name()] = true
		}
		for _, want := range []string{"list", "get", "create", "update", "delete"} {
			if !names[want] {
				t.Errorf("%s is missing subcommand %q", name, want)
			}
		}
	}
}

func TestRouteUpdateFlagsCoverWritableFields(t *testing.T) {
	for _, name := range []string{"name", "hostname", "backend", "port", "namespace", "backend-namespace", "path-prefix", "tls"} {
		if httpRouteUpdateCmd.Flags().Lookup(name) == nil {
			t.Errorf("http-route update missing flag --%s", name)
		}
	}
	for _, cmd := range []*cobra.Command{tcpRouteUpdateCmd, udpRouteUpdateCmd} {
		for _, name := range []string{"name", "port", "backend", "backend-port", "namespace", "backend-namespace"} {
			if cmd.Flags().Lookup(name) == nil {
				t.Errorf("%s missing flag --%s", cmd.Name(), name)
			}
		}
	}
}

// --- route get ---

func TestHTTPRouteGetDecodesUntypedResponse(t *testing.T) {
	rec, out, _, err := runCmd(t, httpRouteGetCmd, []string{"42", "4"}, okBody(httpRouteJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if got := rec.last().path; got != "/api/kubernetes/clusters/42/httproutes/4/" {
		t.Errorf("path = %q", got)
	}
	for _, want := range []string{"web", "example.com", "web-svc", "8080"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestHTTPRouteGetRendersJSON(t *testing.T) {
	_, out, _, err := runCmd(t, httpRouteGetCmd, []string{"42", "4"}, okBody(httpRouteJSON), "json", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if got["backend_service_name"] != "web-svc" {
		t.Errorf("unexpected payload: %v", got)
	}
}

func TestTCPRouteGetDecodesUntypedResponse(t *testing.T) {
	rec, out, _, err := runCmd(t, tcpRouteGetCmd, []string{"42", "6"}, okBody(tcpRouteJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if got := rec.last().path; got != "/api/kubernetes/clusters/42/tcproutes/6/" {
		t.Errorf("path = %q", got)
	}
	if !strings.Contains(out, "5432") {
		t.Errorf("output missing the port:\n%s", out)
	}
}

func TestUDPRouteGetDecodesUntypedResponse(t *testing.T) {
	rec, out, _, err := runCmd(t, udpRouteGetCmd, []string{"42", "7"}, okBody(udpRouteJSON), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if got := rec.last().path; got != "/api/kubernetes/clusters/42/udproutes/7/" {
		t.Errorf("path = %q", got)
	}
	if !strings.Contains(out, "dns-svc") {
		t.Errorf("output missing the backend:\n%s", out)
	}
}

// The detail routes carry no response schema, so phctl decodes the body itself.
// An empty or null body must be reported, never rendered as a zeroed route.
func TestRouteGetRejectsEmptyBody(t *testing.T) {
	cases := map[string]struct {
		cmd  *cobra.Command
		args []string
	}{
		"http": {httpRouteGetCmd, []string{"42", "4"}},
		"tcp":  {tcpRouteGetCmd, []string{"42", "6"}},
		"udp":  {udpRouteGetCmd, []string{"42", "7"}},
	}
	for kind, tc := range cases {
		for _, body := range []string{"", "null"} {
			t.Run(kind+"/body="+body, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("empty body caused a panic: %v", r)
					}
				}()
				_, _, _, err := runCmd(t, tc.cmd, tc.args, okBody(body), "table", "", false)
				if err == nil {
					t.Fatal("expected an error when the server returns no route")
				}
			})
		}
	}
}

func TestRouteGetRejectsMismatchedRoute(t *testing.T) {
	_, _, _, err := runCmd(t, httpRouteGetCmd, []string{"42", "4"},
		okBody(strings.Replace(httpRouteJSON, `"id":4`, `"id":9`, 1)), "table", "", false)
	if err == nil {
		t.Fatal("expected an error when the server answers with a different route")
	}
}

// --- route update ---
//
// PATCH: only the flags the caller passed are sent, so an omitted field keeps
// its current value instead of being reset to a default.

func TestHTTPRouteUpdateSendsOnlyChangedFields(t *testing.T) {
	setFlags(t, httpRouteUpdateCmd, map[string]string{"path-prefix": "/api"})
	response := strings.Replace(httpRouteJSON, `"path_prefix":"/"`, `"path_prefix":"/api"`, 1)
	rec, out, _, err := runCmd(t, httpRouteUpdateCmd, []string{"42", "4"}, okBody(response), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPatch {
		t.Errorf("method = %s, want PATCH", call.method)
	}
	if call.path != "/api/kubernetes/clusters/42/httproutes/4/" {
		t.Errorf("path = %q", call.path)
	}
	if call.body["path_prefix"] != "/api" {
		t.Errorf("body[path_prefix] = %v", call.body["path_prefix"])
	}
	// Anything else in the body would overwrite a field the caller never named.
	for _, key := range []string{"name", "hostnames", "backend_service_name", "backend_service_port",
		"namespace", "backend_namespace", "enable_tls"} {
		if _, present := call.body[key]; present {
			t.Errorf("body carries unset field %q: %v", key, call.body)
		}
	}
	if !strings.Contains(out, "4") {
		t.Errorf("output missing the route ID:\n%s", out)
	}
}

func TestHTTPRouteUpdateSendsHostnameList(t *testing.T) {
	setFlags(t, httpRouteUpdateCmd, map[string]string{"hostname": "example.com"})
	rec, _, _, err := runCmd(t, httpRouteUpdateCmd, []string{"42", "4"}, okBody(httpRouteJSON), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	hostnames, _ := rec.last().body["hostnames"].([]any)
	if len(hostnames) != 1 || hostnames[0] != "example.com" {
		t.Errorf("body[hostnames] = %v", rec.last().body["hostnames"])
	}
}

func TestRouteUpdateRequiresAtLeastOneField(t *testing.T) {
	for _, tc := range []struct {
		cmd  *cobra.Command
		args []string
		body string
	}{
		{httpRouteUpdateCmd, []string{"42", "4"}, httpRouteJSON},
		{tcpRouteUpdateCmd, []string{"42", "6"}, tcpRouteJSON},
		{udpRouteUpdateCmd, []string{"42", "7"}, udpRouteJSON},
	} {
		t.Run(tc.cmd.Parent().Name(), func(t *testing.T) {
			rec, _, _, err := runCmd(t, tc.cmd, tc.args, okBody(tc.body), "table", "", true)
			if err == nil {
				t.Fatal("expected an error when no field flags are given")
			}
			if rec.count() != 0 {
				t.Errorf("empty update still reached the API (%d call(s))", rec.count())
			}
		})
	}
}

func TestHTTPRouteUpdateRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	setFlags(t, httpRouteUpdateCmd, map[string]string{"name": "web"})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty 200 body caused a panic: %v", r)
		}
	}()
	_, _, _, err := runCmd(t, httpRouteUpdateCmd, []string{"42", "4"}, okBody(""), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the server returns no route")
	}
}

func TestTCPRouteUpdateSendsOnlyChangedFields(t *testing.T) {
	setFlags(t, tcpRouteUpdateCmd, map[string]string{"backend-namespace": "database"})
	response := strings.Replace(tcpRouteJSON, `"backend_namespace":"default"`, `"backend_namespace":"database"`, 1)
	rec, _, _, err := runCmd(t, tcpRouteUpdateCmd, []string{"42", "6"}, okBody(response), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPatch || call.path != "/api/kubernetes/clusters/42/tcproutes/6/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if call.body["backend_namespace"] != "database" {
		t.Errorf("body[backend_namespace] = %v", call.body["backend_namespace"])
	}
	for _, key := range []string{"name", "port", "backend_service_name", "backend_service_port", "namespace"} {
		if _, present := call.body[key]; present {
			t.Errorf("body carries unset field %q: %v", key, call.body)
		}
	}
}

func TestUDPRouteUpdateSendsOnlyChangedFields(t *testing.T) {
	setFlags(t, udpRouteUpdateCmd, map[string]string{"port": "5353"})
	rec, _, _, err := runCmd(t, udpRouteUpdateCmd, []string{"42", "7"}, okBody(udpRouteJSON), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPatch || call.path != "/api/kubernetes/clusters/42/udproutes/7/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if call.body["port"] != float64(5353) {
		t.Errorf("body[port] = %v", call.body["port"])
	}
	if _, present := call.body["backend_service_name"]; present {
		t.Errorf("body carries unset field: %v", call.body)
	}
}

func TestRouteUpdateRejectsMismatchedRoute(t *testing.T) {
	setFlags(t, tcpRouteUpdateCmd, map[string]string{"name": "pg"})
	_, _, _, err := runCmd(t, tcpRouteUpdateCmd, []string{"42", "6"},
		okBody(strings.Replace(tcpRouteJSON, `"id":6`, `"id":9`, 1)), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the server answers with a different route")
	}
}

func TestRouteUpdateRejectsUnappliedFields(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		setFlags(t, httpRouteUpdateCmd, map[string]string{"path-prefix": "/api"})
		_, _, _, err := runCmd(t, httpRouteUpdateCmd, []string{"42", "4"}, okBody(httpRouteJSON), "table", "", true)
		if err == nil {
			t.Fatal("expected an error when the HTTP route still reports the old path prefix")
		}
	})
	t.Run("tcp", func(t *testing.T) {
		setFlags(t, tcpRouteUpdateCmd, map[string]string{"port": "15432"})
		_, _, _, err := runCmd(t, tcpRouteUpdateCmd, []string{"42", "6"}, okBody(tcpRouteJSON), "table", "", true)
		if err == nil {
			t.Fatal("expected an error when the TCP route still reports the old external port")
		}
	})
}

// --- existing create commands must survive an empty body too ---

func TestRouteCreateRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	cases := map[string]struct {
		cmd   *cobra.Command
		flags map[string]string
	}{
		"http": {httpRouteCreateCmd, map[string]string{"name": "web", "backend": "web-svc", "port": "8080"}},
		"tcp":  {tcpRouteCreateCmd, map[string]string{"name": "pg", "port": "5432", "backend": "pg-svc", "backend-port": "5432"}},
		"udp":  {udpRouteCreateCmd, map[string]string{"name": "dns", "port": "5353", "backend": "dns-svc", "backend-port": "53"}},
	}
	for kind, tc := range cases {
		t.Run(kind, func(t *testing.T) {
			setFlags(t, tc.cmd, tc.flags)
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("empty 201 body caused a panic: %v", r)
				}
			}()
			_, _, _, err := runCmd(t, tc.cmd, []string{"42"}, reply(http.StatusCreated, ""), "table", "", true)
			if err == nil {
				t.Fatal("expected an error when the server returns no route")
			}
		})
	}
}

// create and update must be able to express the same route. update gained
// --backend-namespace; without it on create, a backend outside the "default"
// namespace can only be reached by creating the route and then replacing it.
func TestRouteCreateCanSetBackendNamespace(t *testing.T) {
	cases := map[string]struct {
		cmd   *cobra.Command
		flags map[string]string
		body  string
	}{
		"http": {httpRouteCreateCmd, map[string]string{
			"name": "web", "backend": "web-svc", "port": "8080",
			"hostname": "example.com", "backend-namespace": "services",
		}, httpRouteJSON},
		"tcp": {tcpRouteCreateCmd, map[string]string{
			"name": "pg", "port": "5432", "backend": "pg-svc",
			"backend-port": "5432", "backend-namespace": "database",
		}, tcpRouteJSON},
		"udp": {udpRouteCreateCmd, map[string]string{
			"name": "dns", "port": "5353", "backend": "dns-svc",
			"backend-port": "53", "backend-namespace": "infra",
		}, udpRouteJSON},
	}
	for kind, tc := range cases {
		t.Run(kind, func(t *testing.T) {
			want := tc.flags["backend-namespace"]
			setFlags(t, tc.cmd, tc.flags)
			rec, _, _, err := runCmd(t, tc.cmd, []string{"42"}, reply(http.StatusCreated, tc.body), "table", "", true)
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			if got := rec.last().body["backend_namespace"]; got != want {
				t.Errorf("body[backend_namespace] = %v, want %q", got, want)
			}
		})
	}
}

// --- route list ---
//
// These endpoints paginate. The commands used to decode a bare JSON array, so
// they failed against the real API on every call; the schema did not describe
// the response, so nothing caught it. Only the structure of the command was
// ever tested.

func routeListPage(kind, body string, next bool) string {
	nextField := "null"
	if next {
		nextField = `"https://api.test/next"`
	}
	return `{"count":2,"next":` + nextField + `,"previous":null,"results":[` + body + `]}`
}

func TestRouteListFollowsPagination(t *testing.T) {
	cases := map[string]struct {
		cmd   *cobra.Command
		path  string
		first string
		want  []string
	}{
		"http": {httpRouteListCmd, "/api/kubernetes/clusters/42/httproutes/", httpRouteJSON, []string{"web", "example.com"}},
		"tcp":  {tcpRouteListCmd, "/api/kubernetes/clusters/42/tcproutes/", tcpRouteJSON, []string{"pg", "5432"}},
		"udp":  {udpRouteListCmd, "/api/kubernetes/clusters/42/udproutes/", udpRouteJSON, []string{"dns", "5353"}},
	}
	for kind, tc := range cases {
		t.Run(kind, func(t *testing.T) {
			second := strings.Replace(tc.first, `"name":"`, `"name":"second-`, 1)
			rec, out, _, err := runCmd(t, tc.cmd, []string{"42"}, func(_ apiCall, n int) (int, string) {
				if n == 0 {
					return http.StatusOK, routeListPage(kind, tc.first, true)
				}
				return http.StatusOK, routeListPage(kind, second, false)
			}, "table", "", false)
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			if rec.count() != 2 {
				t.Fatalf("expected the second page to be fetched, got %d call(s)", rec.count())
			}
			if rec.calls[0].path != tc.path {
				t.Errorf("path = %q, want %q", rec.calls[0].path, tc.path)
			}
			if got := rec.calls[1].query.Get("page"); got != "2" {
				t.Errorf("second call page = %q, want 2", got)
			}
			for _, want := range append(tc.want, "second-") {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q:\n%s", want, out)
				}
			}
		})
	}
}

// The pre-fix shape. Accepting it again would mean the command had gone back to
// reading an unpaginated body.
func TestRouteListRejectsABareArray(t *testing.T) {
	_, _, _, err := runCmd(t, httpRouteListCmd, []string{"42"}, okBody("["+httpRouteJSON+"]"), "table", "", false)
	if err == nil {
		t.Fatal("expected an error: these endpoints answer with a paginated envelope")
	}
}

func TestRouteListRendersJSON(t *testing.T) {
	_, out, _, err := runCmd(t, httpRouteListCmd, []string{"42"},
		okBody(routeListPage("http", httpRouteJSON, false)), "json", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(got) != 1 || got[0]["backend_service_name"] != "web-svc" {
		t.Errorf("unexpected payload: %v", got)
	}
}
