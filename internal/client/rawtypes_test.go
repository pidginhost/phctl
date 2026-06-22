package client

import (
	"encoding/json"
	"testing"
)

// TestRawDomainUnmarshalIdnaBool guards against the regression where the
// API's boolean `idna` flag was decoded into a string field, breaking
// every `domain list`/`domain get` call with
// "cannot unmarshal bool into Go struct field RawDomain.idna of type string".
func TestRawDomainUnmarshalIdnaBool(t *testing.T) {
	// Trimmed but faithful sample from /api/domain/domain/<name>/.
	const body = `{
		"id": 998,
		"domain": "dubify",
		"tld": {"id": 482, "tld": "ro", "price": "7.70", "registrar": "rotld"},
		"idna": false,
		"idna_name": "dubify.ro",
		"nameservers": "ns1.pidginhost.net,ns2.pidginhost.net",
		"expiration_date": "2026-12-10",
		"service_status": "Active",
		"max_renew_years": 9
	}`

	var d RawDomain
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		t.Fatalf("unmarshal RawDomain: %v", err)
	}
	if d.Idna {
		t.Errorf("Idna = %v, want false", d.Idna)
	}
	if d.IdnaName != "dubify.ro" {
		t.Errorf("IdnaName = %q, want %q", d.IdnaName, "dubify.ro")
	}
	if d.Nameservers == nil || *d.Nameservers != "ns1.pidginhost.net,ns2.pidginhost.net" {
		t.Errorf("Nameservers = %v, want the comma-separated list", d.Nameservers)
	}
	if d.Tld.Tld != "ro" {
		t.Errorf("Tld.Tld = %q, want %q", d.Tld.Tld, "ro")
	}
}
