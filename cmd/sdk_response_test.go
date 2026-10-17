package cmd

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Wire fixtures exercise every command that decodes the resources changed in
// SDK v0.15.0. Do not construct them with the SDK: that would hide missing fields.
const sdkBillingJSON = `{"amount":"12.50","currency":"EUR","tax_included":false,
	"cycle_months":1,"cycle_label":"Monthly","next_invoice":"2026-11-01",
	"profile":{"id":3,"name":"Default"},"metered":true,"included":false,
	"hourly_amount":"0.0171","monthly_total":"12.50","components":{"storage":"2.50"}}`

const sdkServerJSON = `{"billing":null,"id":42,"hostname":"vm.example.test",
	"image":"debian","package":"small","cpus":2,"memory":4,"disk_size":40,
	"generation":"standard","status":"active","destroy_protection":false,
	"ha_enabled":false,"custom_os":false,"networks":{"public":{},"private":[]},
	"rescue_mode":false,"boot_iso":null,"rescue_supported":true}`

const sdkVolumeJSON = `{"id":7,"alias":"data","size":50,"product":"ssd",
	"attached":true,"server":"vm.example.test","status":"active","billing":null}`

const sdkNetworkJSON = `{"id":7,"slug":"private","address":"192.0.2.0/24",
	"provisioned":true,"servers":[],"billing":null}`

const sdkBucketJSON = `{"billing":null,"id":7,"name":"assets","full_name":"c1-assets",
	"quota_gb":50,"used_bytes":3221225472,"object_count":2147483648,
	"public_read":false,"status":"active","endpoint":"https://s3.example.test",
	"region":"eu-1","created":"2026-08-18T10:00:00Z"}`

const sdkDedicatedJSON = `{"billing":null,"hardware":null,"id":7,"hostname":"metal.example.test",
	"status":"active","price":"99.00","next_invoice":"2026-11-01",
	"created":"2026-01-01T00:00:00Z","billing_cycle":"monthly",
	"server_status":null,"ips":[],"os_name":null}`

const sdkHardwareJSON = `{"title":"Example server","processor":"Example CPU","processor_count":2,
	"memory_gb":64,"disks":"2 x SSD","raid":"RAID1","network":"1 Gbps",
	"traffic":"Unmetered","gpu":"","extra_hardware":[],"software":"Linux"}`

func TestSDKResourceResponses(t *testing.T) {
	serverDetail := strings.TrimSuffix(sdkServerJSON, "}") +
		`,"machine":{},"volumes":[` + sdkVolumeJSON + `],"floating_ips":[],"username":"debian"}`
	for _, tc := range []struct {
		command, resource, shape, text string
	}{
		{"compute server list", sdkServerJSON, "page", "vm.example.test"},
		{"compute server get 42", serverDetail, "object", "vm.example.test"},
		{"compute volume list", sdkVolumeJSON, "array", "data"},
		{"compute volume get 7", sdkVolumeJSON, "object", "data"},
		{"compute network list", sdkNetworkJSON, "page", "192.0.2.0/24"},
		{"compute network get 7", sdkNetworkJSON, "object", "192.0.2.0/24"},
		{"compute network create --address 192.0.2.0/24", sdkNetworkJSON, "object", "192.0.2.0/24"},
		{"storage bucket list", sdkBucketJSON, "array", "3221225472"},
		{"storage bucket get 7", sdkBucketJSON, "object", "3221225472"},
		{"storage bucket create --name assets --quota 50", sdkBucketJSON, "object", "assets"},
		{"storage bucket resize 7 --quota 50", sdkBucketJSON, "object", "resized"},
		{"storage bucket visibility 7 --private", sdkBucketJSON, "object", "private"},
		{"dedicated server list", sdkDedicatedJSON, "page", "metal.example.test"},
		{"dedicated server get 7", sdkDedicatedJSON, "object", "99.00"},
	} {
		for _, populated := range []bool{false, true} {
			for _, format := range []string{"table", "json"} {
				t.Run(fmt.Sprintf("%s/populated=%t/%s", tc.command, populated, format), func(t *testing.T) {
					resource := tc.resource
					if populated {
						// Replace nested volume billing too, so server detail covers
						// decoding the new required fields in embedded resources.
						resource = strings.ReplaceAll(resource, `"billing":null`, `"billing":`+sdkBillingJSON)
						resource = strings.ReplaceAll(resource, `"hardware":null`, `"hardware":`+sdkHardwareJSON)
					}
					body, want := resource, resource
					switch tc.shape {
					case "array":
						body, want = "["+resource+"]", "["+resource+"]"
					case "page":
						body = `{"count":1,"next":null,"previous":null,"results":[` + resource + `]}`
						want = "[" + resource + "]"
					}
					out, err := runResponseCommand(t, tc.command, format, body)
					if err != nil {
						t.Fatal(err)
					}
					if format == "table" {
						if !strings.Contains(out, tc.text) || strings.Contains(out, "%!") {
							t.Fatalf("table = %q, want %q without format errors", out, tc.text)
						}
						return
					}
					assertResponseJSON(t, out, want)
				})
			}
		}
	}
}

func TestSDKInvoiceLocalAmounts(t *testing.T) {
	for _, invoiceType := range []string{"standard", "usage"} {
		t.Run(invoiceType, func(t *testing.T) {
			body := fmt.Sprintf(`{"id":42,"number_proforma":"PRO-42","number_fiscal":"",
				"status":"unpaid","invoice_type":%q,"can_pay_with_funds":true,
				"subtotal":"125.00","vat_value":"26.25","vat_percentage":21,"total":"151.25",
				"local_amounts":{"currency":"RON","subtotal":"625.00","vat_value":"131.25",
				"total":"756.25","primary":true},"invoice_date":"2026-10-02",
				"due_date":null,"payment_date":null,"product_info":[],"usage_detail":{},
				"client_info":{},"invoice_info":{},"payment_method":"funds","services":[]}`, invoiceType)
			out, err := runResponseCommand(t, "billing invoice get 42", "json", body)
			if err != nil {
				t.Fatal(err)
			}
			assertResponseJSON(t, out, body)
		})
	}
}

func assertResponseJSON(t *testing.T, gotJSON, wantJSON string) {
	t.Helper()
	var got, want any
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatalf("invalid output JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatalf("invalid expected JSON: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("JSON = %s, want %s", gotJSON, wantJSON)
	}
}
