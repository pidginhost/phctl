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

func TestInvoiceGetPreservesDetailsAndDecimals(t *testing.T) {
	const invoice = `{
		"id":42,"number_proforma":"PRO-42","number_fiscal":"INV-42",
		"status":"paid","subtotal":"125.00","vat_value":"26.25",
		"vat_percentage":21,"total":"151.25","invoice_date":"2026-10-02",
		"due_date":null,"payment_date":"2026-10-02","product_info":[],
		"usage_detail":{},"client_info":{},"invoice_info":{},"payment_method":"funds",
		"services":[{"id":7,"hostname":"example.test","status":"active"}]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/billing/invoices/42/" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(invoice))
	}))
	defer server.Close()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "not-a-real-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	for _, format := range []string{"table", "json"} {
		t.Run(format, func(t *testing.T) {
			command := &cobra.Command{}
			command.Flags().String("output", format, "")
			var out bytes.Buffer
			command.SetOut(&out)
			if err := invoiceGetCmd.RunE(command, []string{"42"}); err != nil {
				t.Fatal(err)
			}
			if format == "table" {
				for _, value := range []string{"125.00", "26.25", "151.25"} {
					if !strings.Contains(out.String(), value) {
						t.Errorf("missing exact decimal %q in %q", value, out.String())
					}
				}
				if strings.Contains(out.String(), "%!") {
					t.Errorf("format error: %q", out.String())
				}
				return
			}
			// Compare the wire documents, including service objects and decimal
			// strings, rather than decoding into the same SDK type as production.
			var got, want map[string]any
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(invoice), &want); err != nil {
				t.Fatal(err)
			}
			for key, expected := range want {
				gotJSON, _ := json.Marshal(got[key])
				wantJSON, _ := json.Marshal(expected)
				if !bytes.Equal(gotJSON, wantJSON) {
					t.Errorf("%s = %s, want %s", key, gotJSON, wantJSON)
				}
			}
		})
	}
}
