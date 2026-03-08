package client

// Raw types bypass SDK float64 vs string mismatches for decimal fields.
// The PidginHost API returns decimal values as strings (e.g. "12.50"),
// but the generated SDK expects float64. These types use string fields
// so json.Unmarshal works correctly with RawGet/RawFetchAll.

// --- Account ---

type RawProfile struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Funds     string `json:"funds"`
	Phone     string `json:"phone"`
}

// --- Billing ---

type RawFundsBalance struct {
	Balance       string `json:"balance"`
	ThresholdType string `json:"threshold_type"`
}

type RawDeposit struct {
	Id       int32  `json:"id"`
	Status   string `json:"status"`
	Amount   string `json:"amount"`
	VatValue string `json:"vat_value"`
	Total    string `json:"total"`
	Created  string `json:"created"`
}

type RawInvoiceList struct {
	Id             int32  `json:"id"`
	NumberProforma string `json:"number_proforma"`
	NumberFiscal   string `json:"number_fiscal"`
	Status         string `json:"status"`
	Subtotal       string `json:"subtotal"`
	VatValue       string `json:"vat_value"`
	Total          string `json:"total"`
	InvoiceDate    string `json:"invoice_date"`
	PaymentMethod  string `json:"payment_method"`
}

type RawServiceList struct {
	Id           int32  `json:"id"`
	Hostname     string `json:"hostname"`
	Status       string `json:"status"`
	Price        string `json:"price"`
	NextInvoice  string `json:"next_invoice"`
	BillingCycle string `json:"billing_cycle"`
	AutoPayment  string `json:"auto_payment"`
	Company      string `json:"company"`
}

type RawSubscription struct {
	Id              int32  `json:"id"`
	Status          string `json:"status"`
	ServiceHostname string `json:"service_hostname"`
	Subtotal        string `json:"subtotal"`
	VatValue        string `json:"vat_value"`
	Total           string `json:"total"`
	CreationDate    string `json:"creation_date"`
}

// --- Domain ---

type RawTLD struct {
	Id        int32  `json:"id"`
	Tld       string `json:"tld"`
	Price     string `json:"price"`
	Registrar string `json:"registrar"`
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
	PricePerMonth string  `json:"price_per_month"`
	PricePerHour  string  `json:"price_per_hour"`
	FeaturesReady bool    `json:"features_ready"`
	Ipv4Address   string  `json:"ipv4_address"`
	TalosVersion  string  `json:"talos_version"`
}

// --- Dedicated ---

type RawDedicatedServer struct {
	Id           int32  `json:"id"`
	Hostname     string `json:"hostname"`
	Status       string `json:"status"`
	Price        string `json:"price"`
	NextInvoice  string `json:"next_invoice"`
	Created      string `json:"created"`
	BillingCycle string `json:"billing_cycle"`
	ServerStatus string `json:"server_status"`
	Ips          string `json:"ips"`
	OsName       string `json:"os_name"`
}

// --- Hosting ---

type RawHostingService struct {
	Id           int32  `json:"id"`
	Hostname     string `json:"hostname"`
	Status       string `json:"status"`
	Price        string `json:"price"`
	NextInvoice  string `json:"next_invoice"`
	Created      string `json:"created"`
	BillingCycle string `json:"billing_cycle"`
	PackageName  string `json:"package_name"`
	NodeUrl      string `json:"node_url"`
	Username     string `json:"username"`
}
