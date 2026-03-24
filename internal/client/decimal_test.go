package client

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDecimalUnmarshalJSONString(t *testing.T) {
	// API sends decimal as JSON string: "12.50"
	input := `{"balance":"12.50","threshold_type":"auto"}`
	var bal RawFundsBalance
	if err := json.Unmarshal([]byte(input), &bal); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if bal.Balance != "12.50" {
		t.Errorf("Balance = %q, want %q", bal.Balance, "12.50")
	}
}

func TestDecimalUnmarshalJSONNumber(t *testing.T) {
	// JSON number without quotes: 12.50
	input := `{"balance":12.50,"threshold_type":"auto"}`
	var bal RawFundsBalance
	if err := json.Unmarshal([]byte(input), &bal); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if bal.Balance != "12.50" {
		t.Errorf("Balance = %q, want %q", bal.Balance, "12.50")
	}
}

func TestDecimalMarshalJSON(t *testing.T) {
	bal := RawFundsBalance{Balance: "42.00", ThresholdType: "auto"}
	data, err := json.Marshal(bal)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Should output number without quotes
	want := `{"balance":42.00,"threshold_type":"auto"}`
	if string(data) != want {
		t.Errorf("Marshal = %s, want %s", data, want)
	}
}

func TestDecimalMarshalJSONEmpty(t *testing.T) {
	bal := RawFundsBalance{Balance: "", ThresholdType: "auto"}
	data, err := json.Marshal(bal)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"balance":null,"threshold_type":"auto"}`
	if string(data) != want {
		t.Errorf("Marshal = %s, want %s", data, want)
	}
}

func TestDecimalMarshalYAML(t *testing.T) {
	bal := RawFundsBalance{Balance: "42.50", ThresholdType: "auto"}
	data, err := yaml.Marshal(bal)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// YAML should render decimal as an unquoted number preserving exact
	// precision: 42.50 not 42.5.
	// yaml.v3 uses lowercased Go field names (no yaml tags on this struct).
	got := string(data)
	if got != "balance: 42.50\nthresholdtype: auto\n" {
		t.Errorf("YAML = %q, want %q", got, "balance: 42.50\nthresholdtype: auto\n")
	}
}

func TestDecimalString(t *testing.T) {
	d := Decimal("99.99")
	if d.String() != "99.99" {
		t.Errorf("String() = %q, want %q", d.String(), "99.99")
	}
}

func TestDecimalRoundTrip(t *testing.T) {
	// Simulate: API sends string, we unmarshal, then marshal for output
	input := `{"id":1,"status":"paid","amount":"100.50","vat_value":"19.10","total":"119.60","created":"2024-01-01"}`
	var dep RawDeposit
	if err := json.Unmarshal([]byte(input), &dep); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if dep.Amount != "100.50" {
		t.Errorf("Amount = %q, want %q", dep.Amount, "100.50")
	}

	// Re-marshal
	data, err := json.Marshal(dep)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Re-unmarshal (now from number tokens)
	var dep2 RawDeposit
	if err := json.Unmarshal(data, &dep2); err != nil {
		t.Fatalf("re-Unmarshal: %v", err)
	}
	if dep2.Amount != "100.50" {
		t.Errorf("round-trip Amount = %q, want %q", dep2.Amount, "100.50")
	}
}
