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

func TestRouteUpdateFlagsMirrorCreate(t *testing.T) {
	for _, name := range []string{"name", "hostname", "backend", "port", "namespace", "path-prefix", "tls"} {
		if httpRouteUpdateCmd.Flags().Lookup(name) == nil {
			t.Errorf("http-route update missing flag --%s", name)
		}
	}
	for _, cmd := range []*cobra.Command{tcpRouteUpdateCmd, udpRouteUpdateCmd} {
		for _, name := range []string{"name", "port", "backend", "backend-port", "namespace"} {
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

func TestHTTPRouteUpdateReplacesRoute(t *testing.T) {
	setFlags(t, httpRouteUpdateCmd, map[string]string{
		"name": "web", "backend": "web-svc", "port": "8080", "hostname": "example.com",
		"namespace": "default", "path-prefix": "/api", "tls": "true",
	})

	rec, out, _, err := runCmd(t, httpRouteUpdateCmd, []string{"42", "4"}, okBody(httpRouteJSON), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPut {
		t.Errorf("method = %s, want PUT", call.method)
	}
	if call.path != "/api/kubernetes/clusters/42/httproutes/4/" {
		t.Errorf("path = %q", call.path)
	}
	if call.body["name"] != "web" || call.body["backend_service_name"] != "web-svc" {
		t.Errorf("body = %v", call.body)
	}
	if call.body["path_prefix"] != "/api" {
		t.Errorf("body[path_prefix] = %v", call.body["path_prefix"])
	}
	hostnames, _ := call.body["hostnames"].([]any)
	if len(hostnames) != 1 || hostnames[0] != "example.com" {
		t.Errorf("body[hostnames] = %v", call.body["hostnames"])
	}
	if !strings.Contains(out, "4") {
		t.Errorf("output missing the route ID:\n%s", out)
	}
}

func TestHTTPRouteUpdateRequiresName(t *testing.T) {
	rec, _, _, err := runCmd(t, httpRouteUpdateCmd, []string{"42", "4"}, okBody(httpRouteJSON), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when --name is missing")
	}
	if rec.count() != 0 {
		t.Errorf("incomplete update still reached the API (%d call(s))", rec.count())
	}
}

func TestHTTPRouteUpdateRejectsEmptyBodyWithoutPanicking(t *testing.T) {
	setFlags(t, httpRouteUpdateCmd, map[string]string{
		"name": "web", "backend": "web-svc", "port": "8080", "hostname": "example.com",
	})
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

func TestTCPRouteUpdateReplacesRoute(t *testing.T) {
	setFlags(t, tcpRouteUpdateCmd, map[string]string{
		"name": "pg", "port": "5432", "backend": "pg-svc", "backend-port": "5432", "namespace": "default",
	})
	rec, _, _, err := runCmd(t, tcpRouteUpdateCmd, []string{"42", "6"}, okBody(tcpRouteJSON), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPut || call.path != "/api/kubernetes/clusters/42/tcproutes/6/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if call.body["port"] != float64(5432) || call.body["backend_service_name"] != "pg-svc" {
		t.Errorf("body = %v", call.body)
	}
}

func TestUDPRouteUpdateReplacesRoute(t *testing.T) {
	setFlags(t, udpRouteUpdateCmd, map[string]string{
		"name": "dns", "port": "5353", "backend": "dns-svc", "backend-port": "53", "namespace": "default",
	})
	rec, _, _, err := runCmd(t, udpRouteUpdateCmd, []string{"42", "7"}, okBody(udpRouteJSON), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	call := rec.last()
	if call.method != http.MethodPut || call.path != "/api/kubernetes/clusters/42/udproutes/7/" {
		t.Fatalf("call = %s %s", call.method, call.path)
	}
	if call.body["backend_service_port"] != float64(53) {
		t.Errorf("body = %v", call.body)
	}
}

func TestPortRouteUpdateRequiresNameAndPort(t *testing.T) {
	for _, cmd := range []*cobra.Command{tcpRouteUpdateCmd, udpRouteUpdateCmd} {
		t.Run(cmd.Name(), func(t *testing.T) {
			rec, _, _, err := runCmd(t, cmd, []string{"42", "6"}, okBody(tcpRouteJSON), "table", "", true)
			if err == nil {
				t.Fatal("expected an error when --name and --port are missing")
			}
			if rec.count() != 0 {
				t.Errorf("incomplete update still reached the API (%d call(s))", rec.count())
			}
		})
	}
}

func TestRouteUpdateRejectsMismatchedRoute(t *testing.T) {
	setFlags(t, tcpRouteUpdateCmd, map[string]string{
		"name": "pg", "port": "5432", "backend": "pg-svc", "backend-port": "5432",
	})
	_, _, _, err := runCmd(t, tcpRouteUpdateCmd, []string{"42", "6"},
		okBody(strings.Replace(tcpRouteJSON, `"id":6`, `"id":9`, 1)), "table", "", true)
	if err == nil {
		t.Fatal("expected an error when the server answers with a different route")
	}
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
