package compute

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGatewayUpdatesRespectTextHandling(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cmd      *cobra.Command
		args     []string
		field    string
		response string
		preserve bool
	}{
		{"gateway name", gatewayUpdateCmd, []string{"5"}, "name", gatewayJSON, false},
		{"firewall comment", gatewayFirewallRuleUpdateCmd, []string{"5", "9"}, "comment", gatewayRuleJSON, false},
		{"forward comment", gatewayPortForwardUpdateCmd, []string{"5", "4"}, "comment", gatewayForwardJSON, false},
		{"pool name", gatewayPoolUpdateCmd, []string{"5", "2"}, "name", gatewayPoolJSON, true},
		{"listener comment", gatewayListenerUpdateCmd, []string{"5", "7"}, "comment", gatewayListenerJSON, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, applied := range []bool{true, false} {
				name := "applied"
				if !applied {
					name = "unchanged"
				}
				t.Run(name, func(t *testing.T) {
					setFlags(t, tc.cmd, map[string]string{tc.field: " padded "})
					var response map[string]any
					if err := json.Unmarshal([]byte(tc.response), &response); err != nil {
						t.Fatal(err)
					}
					value := "padded"
					if tc.preserve && applied {
						value = " padded "
					} else if !tc.preserve && !applied {
						value = "old"
					}
					response[tc.field] = value
					st, out, err := runOp(t, tc.cmd, tc.args, okOp(mustJSON(t, response)), "json", "", false)
					if st.body[tc.field] != " padded " {
						t.Fatalf("sent %v; want the supplied text", st.body)
					}
					if applied {
						if err != nil {
							t.Fatalf("successful update rejected: %v", err)
						}
						var got map[string]any
						if err := json.Unmarshal([]byte(out), &got); err != nil || got[tc.field] != value {
							t.Fatalf("output = %q, err = %v; want returned text %q", out, err, value)
						}
					} else if err == nil || !strings.Contains(err.Error(), tc.field+" was not applied") || out != "" {
						t.Fatalf("unchanged response: output = %q, err = %v", out, err)
					}
				})
			}
		})
	}
}
