package e2e

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func isRateLimited(output string) bool {
	return strings.Contains(output, "429") || strings.Contains(output, "Too Many Requests")
}

func run(t *testing.T, args ...string) string {
	t.Helper()
	label := strings.Join(args, " ")
	// Delay between API calls to stay under rate limits.
	time.Sleep(2 * time.Second)
	// Retry up to 3 times on 429 with increasing backoff.
	for attempt := 0; ; attempt++ {
		cmd := exec.Command(binaryPath, args...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out)
		}
		output := string(out)
		// 403: token lacks permission — skip, not fail.
		if strings.Contains(output, "403") || strings.Contains(output, "Forbidden") {
			t.Skipf("phctl %s: endpoint returned 403 (token may lack permission)", label)
		}
		// 429: back off and retry.
		if isRateLimited(output) && attempt < 3 {
			backoff := time.Duration(5*(attempt+1)) * time.Second
			t.Logf("phctl %s: rate limited (429), retrying in %s (attempt %d/3)", label, backoff, attempt+1)
			time.Sleep(backoff)
			continue
		}
		if isRateLimited(output) {
			t.Skipf("phctl %s: still rate limited after 3 retries", label)
		}
		t.Fatalf("phctl %s failed: %v\nOutput:\n%s", label, err, output)
	}
}

// runAllowFail executes a command and returns the output and error without
// failing the test. Retries on rate limits like run does.
func runAllowFail(t *testing.T, args ...string) (string, error) {
	t.Helper()
	time.Sleep(2 * time.Second)
	for attempt := 0; ; attempt++ {
		cmd := exec.Command(binaryPath, args...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), nil
		}
		output := string(out)
		if isRateLimited(output) && attempt < 3 {
			backoff := time.Duration(5*(attempt+1)) * time.Second
			t.Logf("phctl %s: rate limited, retrying in %s (attempt %d/3)", strings.Join(args, " "), backoff, attempt+1)
			time.Sleep(backoff)
			continue
		}
		return output, err
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// --- Server Lifecycle ---

// TestE2E_ComputeServerLifecycle creates a cloud server, waits for it to become
// active, fetches its details, optionally SSHs in to verify the OS, and finally
// destroys it. Gated behind PHCTL_E2E_LIFECYCLE=1 because it provisions real
// (billable) infrastructure.
func TestE2E_ComputeServerLifecycle(t *testing.T) {
	skipWithoutToken(t)
	if os.Getenv("PHCTL_E2E_LIFECYCLE") == "" {
		t.Skip("PHCTL_E2E_LIFECYCLE not set, skipping server lifecycle test (creates real resources)")
	}

	image := envOrDefault("PHCTL_TEST_IMAGE", "ubuntu24")
	pkg := envOrDefault("PHCTL_TEST_PACKAGE", "cloudv-0")
	password := envOrDefault("PHCTL_TEST_PASSWORD", fmt.Sprintf("E2e!T3st%d", time.Now().UnixNano()%100000))
	hostname := fmt.Sprintf("e2e-test-%d", time.Now().UnixNano()%100000)
	provisionTimeout := 5 * time.Minute

	// Step 1: Create server.
	t.Logf("Creating server (image=%s, package=%s, hostname=%s)", image, pkg, hostname)
	out := run(t, "compute", "server", "create",
		"--image", image,
		"--package", pkg,
		"--password", password,
		"--hostname", hostname)

	re := regexp.MustCompile(`ID:\s*(\d+)`)
	matches := re.FindStringSubmatch(out)
	if len(matches) < 2 {
		t.Fatalf("could not parse server ID from create output: %s", out)
	}
	serverID := matches[1]
	t.Logf("Server created (ID: %s)", serverID)

	// Guarantee cleanup: destroy the server even if the test fails.
	t.Cleanup(func() {
		t.Logf("Cleanup: destroying server %s", serverID)
		if out, err := runAllowFail(t, "compute", "server", "delete", serverID, "-f"); err != nil {
			t.Logf("Cleanup warning: could not destroy server %s: %v\n%s", serverID, err, out)
		}
	})

	// Step 2: Poll until the server is active (or timeout).
	t.Log("Waiting for server to become active...")
	var server map[string]interface{}
	deadline := time.Now().Add(provisionTimeout)
	pollInterval := 15 * time.Second
	for time.Now().Before(deadline) {
		out, err := runAllowFail(t, "compute", "server", "get", serverID, "-o", "json")
		if err != nil {
			t.Logf("  get failed (will retry): %v", err)
			time.Sleep(pollInterval)
			continue
		}
		if err := json.Unmarshal([]byte(out), &server); err != nil {
			t.Logf("  JSON parse failed (will retry): %v", err)
			time.Sleep(pollInterval)
			continue
		}
		status, _ := server["status"].(string)
		t.Logf("  status: %s", status)
		if status == "active" {
			break
		}
		if status == "failed" || status == "terminated" || status == "cancelled" {
			t.Fatalf("Server entered terminal status %q", status)
		}
		time.Sleep(pollInterval)
	}

	if server == nil {
		t.Fatal("Could not retrieve server details before timeout")
	}
	status, _ := server["status"].(string)
	if status != "active" {
		t.Fatalf("Server did not become active within %s (last status: %q)", provisionTimeout, status)
	}

	// Step 3: Verify server details match what was requested.
	t.Log("Verifying server details...")
	gotImage, _ := server["image"].(string)
	if !strings.EqualFold(gotImage, image) {
		t.Errorf("Image mismatch: got %q, requested %q", gotImage, image)
	}
	gotPkg, _ := server["package"].(string)
	if !strings.Contains(strings.ToLower(gotPkg), strings.ToLower(pkg)) {
		t.Errorf("Package mismatch: got %q, requested %q", gotPkg, pkg)
	}
	gotHostname, _ := server["hostname"].(string)
	if gotHostname != hostname {
		t.Errorf("Hostname mismatch: got %q, want %q", gotHostname, hostname)
	}
	t.Logf("Server details OK (image=%s, package=%s, hostname=%s)", gotImage, gotPkg, gotHostname)

	// Step 4: SSH in and verify the OS (best-effort).
	ip := extractIPFromNetworks(server)
	if ip == "" {
		t.Log("No public IP found in server networks, skipping SSH verification")
	} else {
		t.Logf("Attempting SSH to %s...", ip)
		verifyOSviaSSH(t, ip, password, image)
	}

	// Step 5: Destroy the server.
	t.Logf("Destroying server %s...", serverID)
	out = run(t, "compute", "server", "delete", serverID, "-f")
	if !strings.Contains(out, "deleted") {
		t.Errorf("Expected deletion confirmation, got: %s", out)
	}
	t.Logf("Server %s destroyed", serverID)
}

// extractIPFromNetworks tries to find a public IPv4 address from the server's
// networks field, which is a map[string]interface{} with an unknown structure.
func extractIPFromNetworks(server map[string]interface{}) string {
	networks, ok := server["networks"].(map[string]interface{})
	if !ok {
		return ""
	}
	// Walk the networks map recursively looking for anything that looks like
	// a public IPv4 address.
	return findIPRecursive(networks)
}

func findIPRecursive(v interface{}) string {
	switch val := v.(type) {
	case string:
		ip := net.ParseIP(strings.TrimSpace(val))
		if ip != nil && ip.To4() != nil && !ip.IsPrivate() && !ip.IsLoopback() {
			return ip.String()
		}
	case map[string]interface{}:
		// Prioritise keys that look like IPv4 / address fields.
		priority := []string{"ipv4", "ip", "address", "public_ip", "main_ip", "ip_address"}
		for _, key := range priority {
			if child, ok := val[key]; ok {
				if found := findIPRecursive(child); found != "" {
					return found
				}
			}
		}
		for _, child := range val {
			if found := findIPRecursive(child); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, child := range val {
			if found := findIPRecursive(child); found != "" {
				return found
			}
		}
	}
	return ""
}

// verifyOSviaSSH connects to the server with sshpass and checks /etc/os-release.
func verifyOSviaSSH(t *testing.T, ip, password, expectedImage string) {
	t.Helper()

	// sshpass is required for non-interactive password-based SSH.
	if _, err := exec.LookPath("sshpass"); err != nil {
		t.Log("sshpass not found in PATH, skipping SSH verification")
		return
	}

	// Give sshd a moment to accept connections after the server goes active.
	time.Sleep(10 * time.Second)

	cmd := exec.Command("sshpass", "-p", password,
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=15",
		"-o", "LogLevel=ERROR",
		"root@"+ip,
		"cat /etc/os-release")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("SSH failed (non-fatal): %v\n%s", err, out)
		return
	}

	osRelease := string(out)
	t.Logf("Remote /etc/os-release (first 500 chars):\n%s", osRelease[:min(len(osRelease), 500)])

	// Derive a loose keyword from the image slug (e.g. "ubuntu-24" → "ubuntu").
	keyword := strings.Split(strings.ToLower(expectedImage), "-")[0]
	if !strings.Contains(strings.ToLower(osRelease), keyword) {
		t.Errorf("OS verification failed: expected %q in os-release, got:\n%s", keyword, osRelease)
	} else {
		t.Logf("OS verified: %q found in /etc/os-release", keyword)
	}
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
