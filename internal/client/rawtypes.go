package client

import (
	"encoding/json"
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Decimal handles API fields that return decimal values as quoted strings
// (e.g. "12.50") instead of JSON numbers. The PidginHost API inconsistently
// returns these as JSON strings, but the generated SDK expects float64.
// Decimal accepts both forms during unmarshal and produces clean output in
// all formats (table, JSON, YAML).
//
// TODO(sdk): Fix the OpenAPI spec to declare these fields as type: string,
// format: decimal. This would let the generated SDK use string fields natively,
// eliminating all Raw* types in this file (~130 lines of workaround).
// See: https://swagger.io/docs/specification/data-models/data-types/
type Decimal string

// UnmarshalJSON accepts both "12.50" (JSON string) and 12.50 (JSON number).
func (d *Decimal) UnmarshalJSON(data []byte) error {
	// Path A: JSON string → e.g. "12.50" (what the real API sends)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*d = Decimal(s)
		return nil
	}
	// Path B: JSON number → e.g. 12.50
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*d = Decimal(n.String())
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into Decimal", string(data))
}

// MarshalJSON outputs as a bare JSON number when possible, preserving the
// original string representation (e.g. "10.00" → 10.00, not 10).
func (d Decimal) MarshalJSON() ([]byte, error) {
	if d == "" {
		return []byte("null"), nil
	}
	if _, err := strconv.ParseFloat(string(d), 64); err == nil {
		return []byte(string(d)), nil
	}
	return json.Marshal(string(d))
}

// MarshalYAML emits the exact decimal string as an unquoted YAML scalar,
// preserving the original precision (e.g. "42.50" stays 42.50, not 42.5).
func (d Decimal) MarshalYAML() (interface{}, error) {
	if d == "" {
		return nil, nil
	}
	// Validate it is actually numeric; if not, fall back to plain string.
	if _, err := strconv.ParseFloat(string(d), 64); err != nil {
		return string(d), nil
	}
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!float",
		Value: string(d),
	}, nil
}

func (d Decimal) String() string {
	return string(d)
}

// --- Account ---

type RawProfile struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Funds     Decimal `json:"funds"`
	Phone     string  `json:"phone"`
}

// --- Billing ---

type RawFundsBalance struct {
	Balance       Decimal `json:"balance"`
	ThresholdType string  `json:"threshold_type"`
}

type RawDeposit struct {
	Id       int32   `json:"id"`
	Status   string  `json:"status"`
	Amount   Decimal `json:"amount"`
	VatValue Decimal `json:"vat_value"`
	Total    Decimal `json:"total"`
	Created  string  `json:"created"`
}

type RawInvoiceList struct {
	Id             int32   `json:"id"`
	NumberProforma string  `json:"number_proforma"`
	NumberFiscal   string  `json:"number_fiscal"`
	Status         string  `json:"status"`
	Subtotal       Decimal `json:"subtotal"`
	VatValue       Decimal `json:"vat_value"`
	Total          Decimal `json:"total"`
	InvoiceDate    string  `json:"invoice_date"`
	PaymentMethod  string  `json:"payment_method"`
}

type RawServiceList struct {
	Id           int32   `json:"id"`
	Hostname     string  `json:"hostname"`
	Status       string  `json:"status"`
	Price        Decimal `json:"price"`
	NextInvoice  string  `json:"next_invoice"`
	BillingCycle string  `json:"billing_cycle"`
	AutoPayment  string  `json:"auto_payment"`
	Company      string  `json:"company"`
}

type RawSubscription struct {
	Id              int32   `json:"id"`
	Status          string  `json:"status"`
	ServiceHostname string  `json:"service_hostname"`
	Subtotal        Decimal `json:"subtotal"`
	VatValue        Decimal `json:"vat_value"`
	Total           Decimal `json:"total"`
	CreationDate    string  `json:"creation_date"`
}

// --- Domain ---

type RawTLD struct {
	Id        int32   `json:"id"`
	Tld       string  `json:"tld"`
	Price     Decimal `json:"price"`
	Registrar string  `json:"registrar"`
}

type RawDomain struct {
	Id             int32   `json:"id"`
	Domain         string  `json:"domain"`
	Idna           string  `json:"idna"`
	Tld            RawTLD  `json:"tld"`
	Nameservers    *string `json:"nameservers"`
	ExpirationDate string  `json:"expiration_date"`
	ServiceStatus  string  `json:"service_status"`
	MaxRenewYears  int32   `json:"max_renew_years"`
}

// --- Kubernetes ---

type RawCluster struct {
	Id            int32   `json:"id"`
	Status        string  `json:"status"`
	Name          *string `json:"name,omitempty"`
	ClusterType   string  `json:"cluster_type"`
	KubeVersion   string  `json:"kube_version"`
	PricePerMonth Decimal `json:"price_per_month"`
	PricePerHour  Decimal `json:"price_per_hour"`
	FeaturesReady bool    `json:"features_ready"`
	Ipv4Address   string  `json:"ipv4_address"`
	TalosVersion  string  `json:"talos_version"`
}

// --- Dedicated ---

type RawDedicatedServer struct {
	Id           int32   `json:"id"`
	Hostname     string  `json:"hostname"`
	Status       string  `json:"status"`
	Price        Decimal `json:"price"`
	NextInvoice  string  `json:"next_invoice"`
	Created      string  `json:"created"`
	BillingCycle string  `json:"billing_cycle"`
	ServerStatus string  `json:"server_status"`
	Ips          string  `json:"ips"`
	OsName       string  `json:"os_name"`
}

// --- Hosting ---

type RawHostingService struct {
	Id           int32   `json:"id"`
	Hostname     string  `json:"hostname"`
	Status       string  `json:"status"`
	Price        Decimal `json:"price"`
	NextInvoice  string  `json:"next_invoice"`
	Created      string  `json:"created"`
	BillingCycle string  `json:"billing_cycle"`
	PackageName  string  `json:"package_name"`
	NodeUrl      string  `json:"node_url"`
	Username     string  `json:"username"`
}
