package compute

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"
)

func writeTestFile(t *testing.T, path, body string) error {
	t.Helper()
	return os.WriteFile(path, []byte(body), 0o600)
}

func TestComputeCommandStructure(t *testing.T) {
	if Cmd.Use != "compute" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "compute")
	}

	aliases := Cmd.Aliases
	if len(aliases) != 1 || aliases[0] != "c" {
		t.Errorf("Aliases = %v, want [c]", aliases)
	}
}

func TestComputeSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}

	expected := []string{"server", "volume", "firewall", "image", "ipv4", "ipv6", "network", "package"}
	for _, want := range expected {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestServerSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range serverCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "get", "create", "delete", "power", "console", "attach-ipv4", "attach-ipv6", "protect", "snapshot"} {
		if !names[want] {
			t.Errorf("server missing subcommand %q", want)
		}
	}
}

func TestServerAliases(t *testing.T) {
	aliases := serverCmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "s" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("server Aliases = %v, want to contain 's'", aliases)
	}
}

func TestServerDeleteAliases(t *testing.T) {
	aliases := serverDeleteCmd.Aliases
	rmFound, destroyFound := false, false
	for _, a := range aliases {
		if a == "rm" {
			rmFound = true
		}
		if a == "destroy" {
			destroyFound = true
		}
	}
	if !rmFound || !destroyFound {
		t.Errorf("server delete Aliases = %v, want to contain 'rm' and 'destroy'", aliases)
	}
}

func TestServerCreateFlags(t *testing.T) {
	for _, name := range []string{"image", "package", "hostname", "project", "ssh-key-id", "password", "new-ipv4", "no-public-ipv4-ack", "user-data", "user-data-file"} {
		if serverCreateCmd.Flags().Lookup(name) == nil {
			t.Errorf("server create missing flag --%s", name)
		}
	}
}

func TestServerCreateNoPublicIPv4AckSendsNoNetworkAcknowledged(t *testing.T) {
	restore := snapshotServerCreateState(t)
	t.Cleanup(restore)

	var (
		gotBody     map[string]interface{}
		requestSeen bool
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestSeen = true
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/cloud/servers/" {
			t.Errorf("path = %s, want /api/cloud/servers/", r.URL.Path)
		}
		if got, want := r.Header.Get("Authorization"), "Token test-token"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123}`))
	}))
	defer server.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	setServerCreateFlag(t, "image", "ubuntu-24.04")
	setServerCreateFlag(t, "package", "starter")
	setServerCreateFlag(t, "no-public-ipv4-ack", "true")

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := serverCreateCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !requestSeen {
		t.Fatal("server create did not call API")
	}
	if got, ok := gotBody["no_network_acknowledged"].(bool); !ok || !got {
		t.Fatalf("no_network_acknowledged = %#v, want true", gotBody["no_network_acknowledged"])
	}
	if _, ok := gotBody["new_ipv4"]; ok {
		t.Fatalf("new_ipv4 was sent even though --new-ipv4 was not set: %#v", gotBody["new_ipv4"])
	}
	if got, want := strings.TrimSpace(out.String()), "Server created (ID: 123)"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestPackageListTableIncludesAvailableGenerations(t *testing.T) {
	var out bytes.Buffer
	printPackageListTable(&out, []pidginhost.ServerProduct{
		{
			Id:                   1,
			Name:                 "Compute-2G",
			Slug:                 "c2g",
			Cpus:                 2,
			Memory:               4,
			DiskSize:             80,
			Traffic:              1000,
			AvailableGenerations: []string{"gen3", "gen4"},
		},
	})

	lines := nonEmptyLines(out.String())
	if len(lines) != 2 {
		t.Fatalf("lines = %#v, want header and one package row", lines)
	}
	assertFields(t, lines[0], []string{"ID", "NAME", "SLUG", "CPUS", "MEMORY_GB", "DISK_GB", "TRAFFIC_GB", "GENERATIONS"})
	assertFields(t, lines[1], []string{"1", "Compute-2G", "c2g", "2", "4", "80", "1000", "gen3,gen4"})
}

func TestFloatingIPListTablesIncludeReverseDNS(t *testing.T) {
	label4 := "edge-v4"
	var out bytes.Buffer
	printFloatingIPv4ListTable(&out, []pidginhost.FloatingIPv4{
		{
			Id:                11,
			Address:           "192.0.2.10",
			ReverseDns:        "edge4.example.com",
			Label:             &label4,
			AuthorizedVmCount: 2,
		},
	})

	lines := nonEmptyLines(out.String())
	if len(lines) != 2 {
		t.Fatalf("IPv4 lines = %#v, want header and one row", lines)
	}
	assertFields(t, lines[0], []string{"ID", "ADDRESS", "LABEL", "REVERSE_DNS", "AUTHORIZED"})
	assertFields(t, lines[1], []string{"11", "192.0.2.10", "edge-v4", "edge4.example.com", "2"})

	label6 := "edge-v6"
	out.Reset()
	printFloatingIPv6ListTable(&out, []pidginhost.FloatingIPv6{
		{
			Id:                12,
			Address:           "2001:db8::10",
			ReverseDns:        "edge6.example.com",
			Label:             &label6,
			AuthorizedVmCount: 3,
		},
	})

	lines = nonEmptyLines(out.String())
	if len(lines) != 2 {
		t.Fatalf("IPv6 lines = %#v, want header and one row", lines)
	}
	assertFields(t, lines[0], []string{"ID", "ADDRESS", "LABEL", "REVERSE_DNS", "AUTHORIZED"})
	assertFields(t, lines[1], []string{"12", "2001:db8::10", "edge-v6", "edge6.example.com", "3"})
}

func TestServerDetailsTableIncludesFloatingIPs(t *testing.T) {
	s := newTestServerDetail([]pidginhost.FloatingIPSummary{
		{
			Id:         99,
			Version:    pidginhost.VERSIONENUM_IPV4,
			Address:    "192.0.2.10",
			Label:      "edge-v4",
			ReverseDns: "edge4.example.com",
		},
	})

	var out bytes.Buffer
	printServerDetailsTable(&out, s)

	lines := nonEmptyLines(out.String())
	section := indexOfLine(lines, "Floating IPs:")
	if section == -1 {
		t.Fatalf("output missing Floating IPs section:\n%s", out.String())
	}
	if section+2 >= len(lines) {
		t.Fatalf("Floating IPs section is incomplete: %#v", lines[section:])
	}
	assertFields(t, lines[section+1], []string{"ID", "VERSION", "ADDRESS", "LABEL", "REVERSE_DNS"})
	assertFields(t, lines[section+2], []string{"99", "ipv4", "192.0.2.10", "edge-v4", "edge4.example.com"})
}

func TestServerDetailsTableHidesEmptyFloatingIPs(t *testing.T) {
	var out bytes.Buffer
	printServerDetailsTable(&out, newTestServerDetail(nil))

	if strings.Contains(out.String(), "Floating IPs:") {
		t.Fatalf("empty floating IP list should be hidden, got:\n%s", out.String())
	}
}

func TestServerPowerFlags(t *testing.T) {
	if serverPowerCmd.Flags().Lookup("action") == nil {
		t.Error("server power missing --action flag")
	}
}

func TestIPv4ReverseDNSEmptyHostnameValidatedBeforeClient(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", "")

	flag := ipv4ReverseDNSCmd.Flags().Lookup("hostname")
	if flag == nil {
		t.Fatal("reverse-dns missing --hostname flag")
	}
	originalValue := flag.Value.String()
	originalChanged := flag.Changed
	t.Cleanup(func() {
		_ = flag.Value.Set(originalValue)
		flag.Changed = originalChanged
	})

	if err := flag.Value.Set(""); err != nil {
		t.Fatalf("set hostname flag: %v", err)
	}
	flag.Changed = true

	err := ipv4ReverseDNSCmd.RunE(ipv4ReverseDNSCmd, []string{"1"})
	if err == nil {
		t.Fatal("expected empty hostname error")
	}
	if got, want := err.Error(), "--hostname requires a non-empty FQDN"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestResolveUserData(t *testing.T) {
	tmp := t.TempDir()

	t.Run("empty returns empty", func(t *testing.T) {
		got, err := resolveUserData("", "", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("inline returns body", func(t *testing.T) {
		got, err := resolveUserData("#!/bin/sh\necho hi", "", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got != "#!/bin/sh\necho hi" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("inline rejects oversize", func(t *testing.T) {
		_, err := resolveUserData(strings.Repeat("a", userDataMaxBytes+1), "", nil)
		if err == nil {
			t.Fatal("expected error for oversize inline")
		}
	})

	t.Run("file path reads body", func(t *testing.T) {
		path := filepath.Join(tmp, "ud.sh")
		body := "#cloud-config\nruncmd:\n  - ls\n"
		if err := writeTestFile(t, path, body); err != nil {
			t.Fatalf("write: %v", err)
		}
		got, err := resolveUserData("", path, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got != body {
			t.Errorf("got %q, want %q", got, body)
		}
	})

	t.Run("file rejects oversize", func(t *testing.T) {
		path := filepath.Join(tmp, "big.sh")
		if err := writeTestFile(t, path, strings.Repeat("a", userDataMaxBytes+1)); err != nil {
			t.Fatalf("write: %v", err)
		}
		_, err := resolveUserData("", path, nil)
		if err == nil {
			t.Fatal("expected error for oversize file")
		}
	})

	t.Run("dash reads provided stdin", func(t *testing.T) {
		body := "#!/bin/sh\necho from stdin\n"
		got, err := resolveUserData("", "-", strings.NewReader(body))
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got != body {
			t.Errorf("got %q, want %q", got, body)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		_, err := resolveUserData("", filepath.Join(tmp, "does-not-exist"), nil)
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestSnapshotSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range snapshotCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "create", "delete", "rollback"} {
		if !names[want] {
			t.Errorf("snapshot missing subcommand %q", want)
		}
	}
}

func newTestServerDetail(floatingIPs []pidginhost.FloatingIPSummary) *pidginhost.ServerDetail {
	return pidginhost.NewServerDetail(
		42,
		"vm.example.com",
		"ubuntu-24.04",
		"c2g",
		2,
		4,
		80,
		"gen3",
		map[string]interface{}{},
		[]pidginhost.Volume{},
		map[string]interface{}{},
		floatingIPs,
		pidginhost.STATUSA57ENUM_ACTIVE,
		"root",
		false,
		true,
	)
}

func nonEmptyLines(s string) []string {
	raw := strings.Split(strings.TrimSpace(s), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func indexOfLine(lines []string, want string) int {
	for i, line := range lines {
		if strings.TrimSpace(line) == want {
			return i
		}
	}
	return -1
}

func assertFields(t *testing.T, line string, want []string) {
	t.Helper()
	got := strings.Fields(line)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields for %q = %#v, want %#v", line, got, want)
	}
}

func setServerCreateFlag(t *testing.T, name, value string) {
	t.Helper()
	if err := serverCreateCmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set --%s: %v", name, err)
	}
}

func snapshotServerCreateState(t *testing.T) func() {
	t.Helper()

	type flagState struct {
		value   string
		changed bool
	}

	flagStates := map[string]flagState{}
	for _, name := range []string{
		"image",
		"package",
		"generation",
		"hostname",
		"project",
		"ssh-key-id",
		"password",
		"new-ipv4",
		"no-public-ipv4-ack",
		"private-network",
		"private-address",
		"user-data",
		"user-data-file",
	} {
		flag := serverCreateCmd.Flags().Lookup(name)
		if flag == nil {
			t.Fatalf("server create missing flag --%s", name)
		}
		flagStates[name] = flagState{value: flag.Value.String(), changed: flag.Changed}
	}

	state := struct {
		image          string
		packageName    string
		generation     string
		hostname       string
		project        string
		sshKeyID       string
		password       string
		newIPv4        bool
		noPubIPv4Ack   bool
		privateNetwork string
		privateAddress string
		userData       string
		userDataFile   string
	}{
		image:          serverCreateImage,
		packageName:    serverCreatePackage,
		generation:     serverCreateGeneration,
		hostname:       serverCreateHostname,
		project:        serverCreateProject,
		sshKeyID:       serverCreateSSHKeyID,
		password:       serverCreatePassword,
		newIPv4:        serverCreateNewIPv4,
		noPubIPv4Ack:   serverCreateNoPubIPv4Ack,
		privateNetwork: serverCreatePrivateNetwork,
		privateAddress: serverCreatePrivateAddress,
		userData:       serverCreateUserData,
		userDataFile:   serverCreateUserDataFile,
	}

	return func() {
		for name, saved := range flagStates {
			flag := serverCreateCmd.Flags().Lookup(name)
			if flag == nil {
				continue
			}
			_ = flag.Value.Set(saved.value)
			flag.Changed = saved.changed
		}
		serverCreateImage = state.image
		serverCreatePackage = state.packageName
		serverCreateGeneration = state.generation
		serverCreateHostname = state.hostname
		serverCreateProject = state.project
		serverCreateSSHKeyID = state.sshKeyID
		serverCreatePassword = state.password
		serverCreateNewIPv4 = state.newIPv4
		serverCreateNoPubIPv4Ack = state.noPubIPv4Ack
		serverCreatePrivateNetwork = state.privateNetwork
		serverCreatePrivateAddress = state.privateAddress
		serverCreateUserData = state.userData
		serverCreateUserDataFile = state.userDataFile
	}
}
