package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var binaryPath string

func TestMain(m *testing.M) {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	dir, err := os.MkdirTemp("", "phctl-e2e-*")
	if err != nil {
		panic(err)
	}
	binaryPath = filepath.Join(dir, "phctl"+ext)

	cmd := exec.Command("go", "build", "-o", binaryPath, "..")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("build phctl: " + err.Error())
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func skipWithoutToken(t *testing.T) {
	t.Helper()
	if os.Getenv("PIDGINHOST_API_TOKEN") == "" {
		t.Skip("PIDGINHOST_API_TOKEN not set, skipping E2E test")
	}
}

func run(t *testing.T, args ...string) string {
	t.Helper()
	// Small delay between API calls to avoid 429 rate limits.
	time.Sleep(500 * time.Millisecond)
	cmd := exec.Command(binaryPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		output := string(out)
		// 403 means the token lacks permission for this endpoint — skip, not fail.
		if strings.Contains(output, "403") || strings.Contains(output, "Forbidden") {
			t.Skipf("phctl %s: endpoint returned 403 (token may lack permission)", strings.Join(args, " "))
		}
		t.Fatalf("phctl %s failed: %v\nOutput:\n%s", strings.Join(args, " "), err, output)
	}
	return string(out)
}

// --- Account ---

func TestE2E_AccountProfile(t *testing.T) {
	skipWithoutToken(t)
	run(t, "account", "profile")
}

func TestE2E_AccountSSHKeyList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "account", "ssh-key", "list")
}

func TestE2E_AccountCompanyList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "account", "company", "list")
}

func TestE2E_AccountAPITokenList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "account", "api-token", "list")
}

func TestE2E_AccountEmailList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "account", "email", "list")
}

// --- Compute ---

func TestE2E_ComputeServerList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "compute", "server", "list")
}

func TestE2E_ComputeVolumeList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "compute", "volume", "list")
}

func TestE2E_ComputeFirewallList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "compute", "firewall", "list")
}

func TestE2E_ComputeImageList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "compute", "image", "list")
}

func TestE2E_ComputePackageList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "compute", "package", "list")
}

func TestE2E_ComputeIPv4List(t *testing.T) {
	skipWithoutToken(t)
	run(t, "compute", "ipv4", "list")
}

func TestE2E_ComputeIPv6List(t *testing.T) {
	skipWithoutToken(t)
	run(t, "compute", "ipv6", "list")
}

func TestE2E_ComputeNetworkList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "compute", "network", "list")
}

// --- Billing ---

func TestE2E_BillingFundsBalance(t *testing.T) {
	skipWithoutToken(t)
	run(t, "billing", "funds", "balance")
}

func TestE2E_BillingFundsLog(t *testing.T) {
	skipWithoutToken(t)
	run(t, "billing", "funds", "log")
}

func TestE2E_BillingDepositList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "billing", "deposit", "list")
}

func TestE2E_BillingInvoiceList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "billing", "invoice", "list")
}

func TestE2E_BillingServiceList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "billing", "service", "list")
}

func TestE2E_BillingSubscriptionList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "billing", "subscription", "list")
}

// --- Domain ---

func TestE2E_DomainList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "domain", "list")
}

func TestE2E_DomainTLDList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "domain", "tld", "list")
}

func TestE2E_DomainRegistrantList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "domain", "registrant", "list")
}

// --- Kubernetes ---

func TestE2E_KubernetesClusterList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "kubernetes", "cluster", "list")
}

// --- Dedicated ---

func TestE2E_DedicatedServerList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "dedicated", "server", "list")
}

// --- FreeDNS ---

func TestE2E_FreeDNSList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "freedns", "domain", "list")
}

// --- Hosting ---

func TestE2E_HostingList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "hosting", "service", "list")
}

// --- Support ---

func TestE2E_SupportDepartmentList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "support", "department", "list")
}

func TestE2E_SupportTicketList(t *testing.T) {
	skipWithoutToken(t)
	run(t, "support", "ticket", "list")
}

// --- Output formats ---

func TestE2E_JSONOutput(t *testing.T) {
	skipWithoutToken(t)
	out := run(t, "billing", "funds", "balance", "-o", "json")
	if !strings.HasPrefix(strings.TrimSpace(out), "[") && !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("expected JSON output, got: %s", out[:min(len(out), 100)])
	}
}

func TestE2E_YAMLOutput(t *testing.T) {
	skipWithoutToken(t)
	out := run(t, "billing", "funds", "balance", "-o", "yaml")
	if strings.HasPrefix(strings.TrimSpace(out), "{") || strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Errorf("expected YAML output, got JSON-like: %s", out[:min(len(out), 100)])
	}
}
