package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestDNSRecordListAddresses(t *testing.T) {
	for _, tc := range []struct{ recordType, address string }{
		{"A", "192.0.2.10"},
		{"AAAA", "2001:db8::10"},
	} {
		for _, format := range []string{"table", "json"} {
			t.Run(tc.recordType+"/"+format, func(t *testing.T) {
				// Keep the fields required by the previous SDK so this also
				// reproduces the rendering regression against that version.
				body := fmt.Sprintf(`[{"Line":1,"name":"www.example.test.","ttl":3600,
					"type":%q,"address":%q,"cname":"","exchange":"","preference":0,
					"txtdata":"","unencoded":"","target":"","priority":0,"weight":0,
					"port":0,"flag":0,"tag":"","value":""}]`, tc.recordType, tc.address)
				out, err := runResponseCommand(t,
					"freedns record list --domain example.test --source internal", format, body)
				if err != nil {
					t.Fatal(err)
				}
				if format == "json" {
					var records []map[string]any
					if err := json.Unmarshal([]byte(out), &records); err != nil {
						t.Fatal(err)
					}
					if len(records) != 1 || records[0]["address"] != tc.address {
						t.Fatalf("address missing from JSON: %s", out)
					}
					return
				}
				want := "LINE TYPE NAME TTL ADDRESS 1 " + tc.recordType + " www.example.test. 3600 " + tc.address
				if got := strings.Join(strings.Fields(out), " "); got != want {
					t.Errorf("table = %q, want %q", got, want)
				}
			})
		}
	}
}

func TestDNSRecordListWithoutAddress(t *testing.T) {
	out, err := runResponseCommand(t,
		"freedns record list --domain example.test --source internal", "table",
		`[{"Line":2,"name":"example.test.","ttl":3600,"type":"TXT","txtdata":"example"}]`)
	if err != nil {
		t.Fatal(err)
	}
	want := "LINE TYPE NAME TTL ADDRESS 2 TXT example.test. 3600 <none>"
	if got := strings.Join(strings.Fields(out), " "); got != want {
		t.Errorf("table = %q, want %q", got, want)
	}
}
