package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewNoToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	_, err := New()
	if err == nil {
		t.Fatal("expected error when no token configured")
	}
	if !strings.Contains(err.Error(), "no API token") {
		t.Errorf("error = %q, want it to mention 'no API token'", err)
	}
}

func TestNewWithToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token-123")
	t.Setenv("PIDGINHOST_API_URL", "")

	client, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewWithConfigFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	// Create config file with token
	configDir := tmp + "/.config/phctl"
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(configDir+"/config.yaml", []byte("auth_token: file-token\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	client, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// --- RawGet / RawFetchAll tests ---

func withHTTPClient(t *testing.T, c *http.Client) {
	t.Helper()
	old := httpClient
	httpClient = c
	t.Cleanup(func() { httpClient = old })
}

func TestRawGetSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/billing/funds/" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RawFundsBalance{Balance: "42.50", ThresholdType: "auto"})
	}))
	defer server.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)
	withHTTPClient(t, server.Client())

	var bal RawFundsBalance
	if err := RawGet("/api/billing/funds/", &bal); err != nil {
		t.Fatalf("RawGet error: %v", err)
	}
	if bal.Balance != "42.50" {
		t.Errorf("Balance = %q, want %q", bal.Balance, "42.50")
	}
	if bal.ThresholdType != "auto" {
		t.Errorf("ThresholdType = %q, want %q", bal.ThresholdType, "auto")
	}
}

func TestRawGetNoToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	var dst RawFundsBalance
	err := RawGet("/api/billing/funds/", &dst)
	if err == nil {
		t.Fatal("expected error when no token configured")
	}
	if !strings.Contains(err.Error(), "no API token") {
		t.Errorf("error = %q, want it to mention 'no API token'", err)
	}
}

func TestRawGetHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)
	withHTTPClient(t, server.Client())

	var dst RawFundsBalance
	err := RawGet("/api/billing/funds/", &dst)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want it to mention status code", err)
	}
}

func TestRawFetchAllSinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(PaginatedResponse[RawDeposit]{
			Count: 2,
			Results: []RawDeposit{
				{Id: 1, Amount: "10.00", Status: "paid"},
				{Id: 2, Amount: "20.00", Status: "paid"},
			},
			Next: nil,
		})
	}))
	defer server.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)
	withHTTPClient(t, server.Client())

	deposits, err := RawFetchAll[RawDeposit]("/api/billing/deposits/")
	if err != nil {
		t.Fatalf("RawFetchAll error: %v", err)
	}
	if len(deposits) != 2 {
		t.Fatalf("got %d deposits, want 2", len(deposits))
	}
	if deposits[0].Amount != "10.00" {
		t.Errorf("deposits[0].Amount = %q, want %q", deposits[0].Amount, "10.00")
	}
}

func TestRawFetchAllMultiplePages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			next := "page2"
			_ = json.NewEncoder(w).Encode(PaginatedResponse[RawDeposit]{
				Count:   3,
				Results: []RawDeposit{{Id: 1, Amount: "10.00"}},
				Next:    &next,
			})
		case "2":
			_ = json.NewEncoder(w).Encode(PaginatedResponse[RawDeposit]{
				Count:   3,
				Results: []RawDeposit{{Id: 2, Amount: "20.00"}, {Id: 3, Amount: "30.00"}},
				Next:    nil,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)
	withHTTPClient(t, server.Client())

	deposits, err := RawFetchAll[RawDeposit]("/api/billing/deposits/")
	if err != nil {
		t.Fatalf("RawFetchAll error: %v", err)
	}
	if len(deposits) != 3 {
		t.Fatalf("got %d deposits, want 3", len(deposits))
	}
	if deposits[2].Id != 3 {
		t.Errorf("deposits[2].Id = %d, want 3", deposits[2].Id)
	}
}

func TestRawFetchAllNoToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	_, err := RawFetchAll[RawDeposit]("/api/billing/deposits/")
	if err == nil {
		t.Fatal("expected error when no token configured")
	}
}
