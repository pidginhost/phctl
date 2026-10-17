package kubernetes

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"
)

func TestDeleteRequestsDoNotSendNull(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cmd       *cobra.Command
		args      []string
		code      int
		body      string
		wantError string
		hostname  string
	}{
		{"pool", poolDeleteCmd, []string{"42", "3"}, http.StatusNoContent, "", "", ""},
		{"node", nodeDeleteCmd, []string{"42", "3", "11"}, http.StatusAccepted, nodeOperationJSON, "", "node-a"},
		{"pool refusal", poolDeleteCmd, []string{"42", "3"}, http.StatusBadRequest,
			`{"detail":"Local data loss acknowledgement is required."}`, "Local data loss acknowledgement is required.", ""},
		{"node refusal", nodeDeleteCmd, []string{"42", "3", "11"}, http.StatusBadRequest,
			`{"detail":"Hostname confirmation does not match."}`, "Hostname confirmation does not match.", "node-b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.hostname != "" {
				setFlags(t, tc.cmd, map[string]string{
					"confirm-hostname":       tc.hostname,
					"accept-local-data-loss": "true",
				})
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodDelete {
					t.Errorf("method = %s, want DELETE", r.Method)
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				// The API reads deletion options from a JSON object.
				var options map[string]any
				if err := json.Unmarshal(raw, &options); err != nil || options == nil {
					t.Errorf("deletion body = %q, want a JSON object", raw)
				}
				if accepted, _ := options["local_data_loss_accepted"].(bool); accepted != (tc.hostname != "") {
					t.Error("deletion must send only the supplied data loss acknowledgement")
				}
				if hostname, _ := options["confirm_hostname"].(string); hostname != tc.hostname {
					t.Errorf("hostname = %q, want supplied hostname %q", hostname, tc.hostname)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			original := newClient
			newClient = func() (*pidginhost.APIClient, error) {
				return pidginhost.New("not-a-real-token", server.URL), nil
			}
			t.Cleanup(func() { newClient = original })
			command := &cobra.Command{}
			command.Flags().Bool("force", true, "")
			var out bytes.Buffer
			command.SetOut(&out)
			err := tc.cmd.RunE(command, tc.args)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want API refusal %q", err, tc.wantError)
				}
				if out.Len() != 0 {
					t.Errorf("refused deletion printed success: %s", out.String())
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Errorf("API calls = %d, want 1", calls)
			}
		})
	}
}

func TestPoolDeleteSendsDataLossAcknowledgement(t *testing.T) {
	setFlags(t, poolDeleteCmd, map[string]string{"accept-local-data-loss": "true"})
	rec, _, _, err := runCmd(t, poolDeleteCmd, []string{"42", "3"},
		reply(http.StatusNoContent, ""), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if got := rec.last().body["local_data_loss_accepted"]; got != true {
		t.Errorf("local_data_loss_accepted = %v, want true", got)
	}
}

func TestNodeDeleteSendsHostnameAndAcknowledgement(t *testing.T) {
	setFlags(t, nodeDeleteCmd, map[string]string{
		"confirm-hostname":       "node-a",
		"accept-local-data-loss": "true",
	})
	rec, _, _, err := runCmd(t, nodeDeleteCmd, []string{"42", "3", "11"},
		reply(http.StatusAccepted, nodeOperationJSON), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	body := rec.last().body
	if body["confirm_hostname"] != "node-a" {
		t.Errorf("confirm_hostname = %v, want node-a", body["confirm_hostname"])
	}
	if body["local_data_loss_accepted"] != true {
		t.Errorf("local_data_loss_accepted = %v, want true", body["local_data_loss_accepted"])
	}
}

// The API refuses every node delete without the typed-back hostname, so the
// CLI asks for it before the confirmation prompt rather than after.
func TestNodeDeleteRequiresHostname(t *testing.T) {
	setFlags(t, nodeDeleteCmd, map[string]string{"accept-local-data-loss": "true"})
	err := nodeDeleteCmd.ValidateRequiredFlags()
	if err == nil || !strings.Contains(err.Error(), "confirm-hostname") {
		t.Errorf("error = %v, want --confirm-hostname required", err)
	}
}

func TestNodeDeleteRequiresDataLossAcknowledgement(t *testing.T) {
	setFlags(t, nodeDeleteCmd, map[string]string{"confirm-hostname": "node-a"})
	err := nodeDeleteCmd.ValidateRequiredFlags()
	if err == nil || !strings.Contains(err.Error(), "accept-local-data-loss") {
		t.Errorf("error = %v, want --accept-local-data-loss required", err)
	}
}

func TestNodeDeleteRejectsMissingConfirmationsBeforePrompt(t *testing.T) {
	for _, tc := range []struct {
		name      string
		flags     map[string]string
		wantError string
	}{
		{"missing hostname", map[string]string{"accept-local-data-loss": "true"}, "--confirm-hostname"},
		{"empty hostname", map[string]string{"confirm-hostname": "", "accept-local-data-loss": "true"}, "--confirm-hostname"},
		{"blank hostname", map[string]string{"confirm-hostname": " \t\n", "accept-local-data-loss": "true"}, "--confirm-hostname"},
		{"missing acknowledgement", map[string]string{"confirm-hostname": "node-a"}, "--accept-local-data-loss"},
		{"false acknowledgement", map[string]string{"confirm-hostname": "node-a", "accept-local-data-loss": "false"}, "--accept-local-data-loss"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setFlags(t, nodeDeleteCmd, tc.flags)
			for _, force := range []bool{false, true} {
				rec, out, errOut, err := runCmd(t, nodeDeleteCmd, []string{"42", "3", "11"},
					reply(http.StatusAccepted, nodeOperationJSON), "table", "y\n", force)
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Errorf("force=%t: error = %v, want %s", force, err, tc.wantError)
				}
				if rec.count() != 0 {
					t.Errorf("force=%t: API calls = %d, want none", force, rec.count())
				}
				if out != "" || errOut != "" {
					t.Errorf("force=%t: invalid input printed output or prompted: stdout=%q stderr=%q", force, out, errOut)
				}
			}
		})
	}
}

func TestNodeDeletePreservesExactHostname(t *testing.T) {
	for _, hostname := range []string{"node-a", "Node-A", " node-a", "node-a "} {
		t.Run(hostname, func(t *testing.T) {
			setFlags(t, nodeDeleteCmd, map[string]string{
				"confirm-hostname":       hostname,
				"accept-local-data-loss": "true",
			})
			rec, out, _, err := runCmd(t, nodeDeleteCmd, []string{"42", "3", "11"},
				func(call apiCall, _ int) (int, string) {
					if call.body["confirm_hostname"] != "node-a" {
						return http.StatusBadRequest, `{"detail":"Hostname confirmation does not match."}`
					}
					return http.StatusAccepted, nodeOperationJSON
				}, "table", "", true)
			if rec.count() != 1 || rec.last().method != http.MethodDelete {
				t.Fatalf("requests = %+v, want only the deletion request", rec.calls)
			}
			if got := rec.last().body["confirm_hostname"]; got != hostname {
				t.Errorf("confirm_hostname = %q, want unmodified input %q", got, hostname)
			}
			if hostname == "node-a" {
				if err != nil {
					t.Fatalf("RunE: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "Hostname confirmation does not match.") {
					t.Errorf("error = %v, want API hostname refusal", err)
				}
				if out != "" {
					t.Errorf("refused deletion printed success: %q", out)
				}
			}
		})
	}
}

func TestDeleteConfirmationsCanBeCancelled(t *testing.T) {
	setFlags(t, nodeDeleteCmd, map[string]string{
		"confirm-hostname":       "node-a",
		"accept-local-data-loss": "true",
	})
	setFlags(t, poolDeleteCmd, map[string]string{"accept-local-data-loss": "true"})
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{"node", nodeDeleteCmd, []string{"42", "3", "11"}},
		{"pool", poolDeleteCmd, []string{"42", "3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, stdin := range []string{"n\n", ""} {
				rec, out, errOut, err := runCmd(t, tc.cmd, tc.args,
					reply(http.StatusNoContent, ""), "table", stdin, false)
				if err != nil || rec.count() != 0 || out != "" {
					t.Errorf("cancelled deletion: error=%v, requests=%d, stdout=%q", err, rec.count(), out)
				}
				if !strings.Contains(errOut, "[y/N]") {
					t.Errorf("delete did not confirm: %q", errOut)
				}
			}
		})
	}
}
