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
	}{
		{"pool", poolDeleteCmd, []string{"42", "3"}, http.StatusNoContent, "", ""},
		{"node", nodeDeleteCmd, []string{"42", "3", "11"}, http.StatusAccepted, nodeOperationJSON, ""},
		{"pool refusal", poolDeleteCmd, []string{"42", "3"}, http.StatusBadRequest,
			`{"detail":"Local data loss acknowledgement is required."}`, "Local data loss acknowledgement is required."},
		{"node refusal", nodeDeleteCmd, []string{"42", "3", "11"}, http.StatusBadRequest,
			`{"detail":"Hostname confirmation is required."}`, "Hostname confirmation is required."},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
				// No body was sent by the previous SDK. If a body is sent,
				// it must be an object: the API reads deletion options from it.
				if len(bytes.TrimSpace(raw)) > 0 {
					var options map[string]any
					if err := json.Unmarshal(raw, &options); err != nil || options == nil {
						t.Errorf("deletion body = %q, want a JSON object or no body", raw)
					}
					if accepted, _ := options["local_data_loss_accepted"].(bool); accepted {
						t.Error("deletion must not silently acknowledge local data loss")
					}
					if hostname, _ := options["confirm_hostname"].(string); hostname != "" {
						t.Error("deletion must not silently confirm the node hostname")
					}
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
	err := nodeDeleteCmd.ValidateRequiredFlags()
	if err == nil || !strings.Contains(err.Error(), "confirm-hostname") {
		t.Errorf("error = %v, want --confirm-hostname required", err)
	}
}
