package billing

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// The API sends every decimal as a JSON string ("125.00"). sdk-go maps those to
// string from v0.12.0, so the %.2f this command used to render Amount with
// produced `%!f(string=125.00)` -- after the deposit had already been created.
func runDepositCreate(t *testing.T, format string) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/billing/deposits/" {
			t.Errorf("path = %s, want /api/billing/deposits/", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"status":"unpaid","amount":"125.00",` +
			`"vat_value":"23.75","vat_percentage":19,"total":"148.75",` +
			`"created":"2026-08-18T10:00:00Z"}`))
	}))
	t.Cleanup(server.Close)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	prev := depositCreateAmount
	t.Cleanup(func() { depositCreateAmount = prev })
	depositCreateAmount = 125

	root := &cobra.Command{Use: "phctl"}
	root.PersistentFlags().StringP("output", "o", format, "Output format")
	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)

	var out bytes.Buffer
	child.SetOut(&out)
	if err := depositCreateCmd.RunE(child, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	return out.String()
}

func TestDepositCreateRendersAmountAsSent(t *testing.T) {
	out := runDepositCreate(t, "table")

	if strings.Contains(out, "%!") {
		t.Errorf("output has a format-verb error: %q", out)
	}
	if !strings.Contains(out, "125.00") {
		t.Errorf("output = %q, want it to contain the amount 125.00", out)
	}
}

func TestDepositCreateJSONCarriesTheDecimalUnchanged(t *testing.T) {
	out := runDepositCreate(t, "json")

	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	// A float round-trip would turn "125.00" into 125; the string must survive.
	if got["amount"] != "125.00" {
		t.Errorf("amount = %#v, want the string \"125.00\"", got["amount"])
	}
}
