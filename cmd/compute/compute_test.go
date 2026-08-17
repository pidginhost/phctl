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
	for _, name := range []string{"image", "package", "hostname", "project", "ssh-key-id", "password", "new-ipv4", "public-ip", "new-ipv6", "public-ipv6", "no-public-ipv4-ack", "user-data", "user-data-file"} {
		if serverCreateCmd.Flags().Lookup(name) == nil {
			t.Errorf("server create missing flag --%s", name)
		}
	}
}

func TestServerAttachFlags(t *testing.T) {
	for _, tc := range []struct {
		name        string
		cmdObj      *cobra.Command
		addressFlag string
	}{
		{name: "ipv4", cmdObj: serverAttachIPv4Cmd, addressFlag: "ipv4"},
		{name: "ipv6", cmdObj: serverAttachIPv6Cmd, addressFlag: "ipv6"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cmdObj.Flags().Lookup(tc.addressFlag) == nil {
				t.Fatalf("%s missing flag --%s", tc.cmdObj.Name(), tc.addressFlag)
			}
			reboot := tc.cmdObj.Flags().Lookup("reboot")
			if reboot == nil {
				t.Fatalf("%s missing flag --reboot", tc.cmdObj.Name())
			}
			if reboot.DefValue != "false" {
				t.Fatalf("%s --reboot default = %q, want false", tc.cmdObj.Name(), reboot.DefValue)
			}
		})
	}
}

// runServerCreate drives serverCreateCmd against a stub API and returns the
// decoded request body plus whatever the command wrote to stdout.
func runServerCreate(t *testing.T, flags map[string]string) (map[string]interface{}, string) {
	t.Helper()
	return runServerCreateFormat(t, flags, "table")
}

// runServerCreateFormat is runServerCreate with the global -o/--output flag set,
// so tests can assert what each format actually emits.
func runServerCreateFormat(t *testing.T, flags map[string]string, format string) (map[string]interface{}, string) {
	t.Helper()

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
	t.Cleanup(server.Close)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	for name, value := range flags {
		setServerCreateFlag(t, name, value)
	}

	cmd := newFakeRootChild(t, format)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := serverCreateCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !requestSeen {
		t.Fatal("server create did not call API")
	}
	return gotBody, out.String()
}

// newFakeRootChild returns a command wired under a throwaway root carrying the
// global -o/--output flag, so cmdutil.OutputFormat resolves during RunE-level
// tests the same way it does under the real root command.
func newFakeRootChild(t *testing.T, format string) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "phctl"}
	root.PersistentFlags().StringP("output", "o", format, "Output format")
	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)
	return child
}

func TestServerCreatePublicIPSendsPublicIP(t *testing.T) {
	body, _ := runServerCreate(t, map[string]string{
		"image":     "ubuntu-24.04",
		"package":   "starter",
		"public-ip": "203.0.113.7",
	})

	if got, ok := body["public_ip"].(string); !ok || got != "203.0.113.7" {
		t.Fatalf("public_ip = %#v, want %q", body["public_ip"], "203.0.113.7")
	}
	if _, ok := body["new_ipv4"]; ok {
		t.Fatalf("new_ipv4 was sent alongside --public-ip: %#v", body["new_ipv4"])
	}
}

func TestServerCreatePublicIPv6SendsPublicIPv6(t *testing.T) {
	body, _ := runServerCreate(t, map[string]string{
		"image":       "ubuntu-24.04",
		"package":     "starter",
		"public-ipv6": "2001:db8::10",
	})

	if got, ok := body["public_ipv6"].(string); !ok || got != "2001:db8::10" {
		t.Fatalf("public_ipv6 = %#v, want %q", body["public_ipv6"], "2001:db8::10")
	}
	if _, ok := body["new_ipv6"]; ok {
		t.Fatalf("new_ipv6 was sent alongside --public-ipv6: %#v", body["new_ipv6"])
	}
}

func TestServerCreateNewIPv6SendsNewIPv6(t *testing.T) {
	body, _ := runServerCreate(t, map[string]string{
		"image":    "ubuntu-24.04",
		"package":  "starter",
		"new-ipv6": "true",
	})

	if got, ok := body["new_ipv6"].(bool); !ok || !got {
		t.Fatalf("new_ipv6 = %#v, want true", body["new_ipv6"])
	}
	if _, ok := body["public_ipv6"]; ok {
		t.Fatalf("public_ipv6 was sent even though --public-ipv6 was not set: %#v", body["public_ipv6"])
	}
}

// TestServerCreateIPFlagsAreMutuallyExclusive drives the real cobra tree so the
// flag-group validation actually runs; RunE-level tests bypass it entirely.
func TestServerCreateIPFlagsAreMutuallyExclusive(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"ipv4", []string{"server", "create", "--image", "ubuntu-24.04", "--package", "starter", "--public-ip", "203.0.113.7", "--new-ipv4"}},
		{"ipv6", []string{"server", "create", "--image", "ubuntu-24.04", "--package", "starter", "--public-ipv6", "2001:db8::10", "--new-ipv6"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := snapshotServerCreateState(t)
			t.Cleanup(restore)

			var out, errOut bytes.Buffer
			Cmd.SetOut(&out)
			Cmd.SetErr(&errOut)
			Cmd.SetArgs(tc.args)
			t.Cleanup(func() {
				Cmd.SetArgs(nil)
				Cmd.SetOut(nil)
				Cmd.SetErr(nil)
			})

			err := Cmd.Execute()
			if err == nil {
				t.Fatal("expected mutually-exclusive flag error, got nil")
			}
			if !strings.Contains(err.Error(), "none of the others can be") {
				t.Fatalf("error = %q, want a mutually-exclusive flag error", err.Error())
			}
		})
	}
}

func TestServerCreateRunERejectsMutuallyExclusiveIPFlags(t *testing.T) {
	for _, tc := range []struct {
		name    string
		flags   map[string]string
		wantErr string
	}{
		{
			name: "ipv4",
			flags: map[string]string{
				"new-ipv4":  "true",
				"public-ip": "203.0.113.7",
			},
			wantErr: "--new-ipv4 and --public-ip are mutually exclusive",
		},
		{
			name: "ipv6",
			flags: map[string]string{
				"new-ipv6":    "true",
				"public-ipv6": "2001:db8::10",
			},
			wantErr: "--new-ipv6 and --public-ipv6 are mutually exclusive",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := snapshotServerCreateState(t)
			t.Cleanup(restore)

			for name, value := range tc.flags {
				setServerCreateFlag(t, name, value)
			}

			err := serverCreateCmd.RunE(&cobra.Command{}, nil)
			if err == nil {
				t.Fatal("expected mutually-exclusive flag error, got nil")
			}
			if err.Error() != tc.wantErr {
				t.Fatalf("error = %q, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestServerCreateNoPublicIPv4AckSendsNoNetworkAcknowledged(t *testing.T) {
	gotBody, out := runServerCreate(t, map[string]string{
		"image":              "ubuntu-24.04",
		"package":            "starter",
		"no-public-ipv4-ack": "true",
	})

	if got, ok := gotBody["no_network_acknowledged"].(bool); !ok || !got {
		t.Fatalf("no_network_acknowledged = %#v, want true", gotBody["no_network_acknowledged"])
	}
	if _, ok := gotBody["new_ipv4"]; ok {
		t.Fatalf("new_ipv4 was sent even though --new-ipv4 was not set: %#v", gotBody["new_ipv4"])
	}
	if got, want := out, "Server created (ID: 123)\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

// TestServerCreateHonoursOutputFlag pins the -o contract: a command that
// returns a resource must emit that resource, not a human sentence, under
// -o json/yaml. `server create` previously printed via cmd.Printf and ignored
// the flag entirely.
func TestServerCreateHonoursOutputFlag(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		_, out := runServerCreateFormat(t, map[string]string{
			"image":   "ubuntu-24.04",
			"package": "starter",
		}, "json")

		var decoded struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal([]byte(out), &decoded); err != nil {
			t.Fatalf("-o json output is not valid JSON (%v): %q", err, out)
		}
		if decoded.ID != 123 {
			t.Errorf("id = %d, want 123", decoded.ID)
		}
	})

	t.Run("table still prints the sentence", func(t *testing.T) {
		_, out := runServerCreateFormat(t, map[string]string{
			"image":   "ubuntu-24.04",
			"package": "starter",
		}, "table")

		if got, want := out, "Server created (ID: 123)\n"; got != want {
			t.Fatalf("output = %q, want %q", got, want)
		}
	})
}

// TestAttachResultMessage pins what the operator is told about reachability.
// A public address is written to the machine config and read by the guest OS
// only at boot, so on a running server a bare "attached" describes a machine
// that answers neither ping nor SSH on the new address.
func TestAttachResultMessage(t *testing.T) {
	for _, tc := range []struct {
		name           string
		rebootRequired bool
		rebooted       bool
		want           []string
		absent         []string
	}{
		{
			name:   "stopped server needs no restart",
			want:   []string{"IPv4 203.0.113.7 attached to server 5."},
			absent: []string{"Restart required", "restarted"},
		},
		{
			name:           "running server is told a restart is owed",
			rebootRequired: true,
			want: []string{
				"IPv4 203.0.113.7 attached to server 5.",
				"Restart required",
				"--reboot",
				"phctl compute server power 5 --action reboot",
			},
		},
		{
			name:     "restart already issued",
			rebooted: true,
			want: []string{
				"IPv4 203.0.113.7 attached to server 5.",
				"restarted",
			},
			absent: []string{"Restart required"},
		},
		{
			name:           "required restart takes precedence over conflicting success",
			rebootRequired: true,
			rebooted:       true,
			want:           []string{"Restart required", "--reboot"},
			absent:         []string{"Server restarted"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := attachResultMessage("IPv4", "203.0.113.7", 5, tc.rebootRequired, tc.rebooted)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("message %q missing %q", got, want)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(got, absent) {
					t.Errorf("message %q should not mention %q", got, absent)
				}
			}
			if !strings.HasSuffix(got, "\n") {
				t.Errorf("message %q should end in a newline", got)
			}
		})
	}
}

// runAttach drives an attach subcommand against a stub API and returns the
// decoded request body and stdout.
func runAttach(t *testing.T, cmdObj *cobra.Command, flags map[string]string, respBody, format string) (map[string]interface{}, string, error) {
	t.Helper()

	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(server.Close)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIDGINHOST_API_TOKEN", "test-token")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	child := newFakeRootChild(t, format)
	for _, name := range []string{"ipv4", "ipv6"} {
		if cmdObj.Flags().Lookup(name) != nil {
			child.Flags().String(name, "", "address")
		}
	}
	child.Flags().Bool("reboot", false, "restart server")
	for name, value := range flags {
		if child.Flags().Lookup(name) == nil {
			t.Fatalf("missing flag --%s", name)
		}
		if err := child.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s: %v", name, err)
		}
	}

	var out bytes.Buffer
	child.SetOut(&out)

	err := cmdObj.RunE(child, []string{"5"})
	return gotBody, out.String(), err
}

func TestServerAttachSendsRebootAndReportsRestart(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cmdObj  *cobra.Command
		flag    string
		addr    string
		bodyKey string
	}{
		{"ipv4", serverAttachIPv4Cmd, "ipv4", "203.0.113.7", "ipv4"},
		{"ipv6", serverAttachIPv6Cmd, "ipv6", "2001:db8::10", "ipv6"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, out, err := runAttach(t, tc.cmdObj,
				map[string]string{tc.flag: tc.addr, "reboot": "true"},
				`{"attached":true,"reboot_required":false,"rebooted":true}`, "table")
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			if got, ok := body["reboot"].(bool); !ok || !got {
				t.Fatalf("reboot = %#v, want true", body["reboot"])
			}
			if got, ok := body[tc.bodyKey].(string); !ok || got != tc.addr {
				t.Fatalf("%s = %#v, want %q", tc.bodyKey, body[tc.bodyKey], tc.addr)
			}
			if !strings.Contains(out, "restarted") {
				t.Errorf("output %q should report the restart", out)
			}
		})
	}
}

func TestServerAttachSendsFalseWhenRebootNotRequested(t *testing.T) {
	body, out, err := runAttach(t, serverAttachIPv4Cmd,
		map[string]string{"ipv4": "203.0.113.7"},
		`{"attached":true,"reboot_required":true,"rebooted":false}`, "table")
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	// The generated request type carries the schema default, so `reboot` is
	// always present on the wire. What must never happen is it going out true
	// without the operator asking -- that would restart a live server.
	if got, ok := body["reboot"].(bool); !ok || got {
		t.Fatalf("reboot = %#v, want false when --reboot is not passed", body["reboot"])
	}
	if !strings.Contains(out, "Restart required") {
		t.Errorf("output %q must warn that the guest cannot see the address yet", out)
	}
}

func TestServerAttachRebootFlagDoesNotLeakBetweenRuns(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cmdObj *cobra.Command
		flag   string
		addr   string
	}{
		{"ipv4", serverAttachIPv4Cmd, "ipv4", "203.0.113.7"},
		{"ipv6", serverAttachIPv6Cmd, "ipv6", "2001:db8::10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := runAttach(t, tc.cmdObj,
				map[string]string{tc.flag: tc.addr, "reboot": "true"},
				`{"attached":true,"reboot_required":false,"rebooted":true}`, "table")
			if err != nil {
				t.Fatalf("first RunE: %v", err)
			}

			body, _, err := runAttach(t, tc.cmdObj,
				map[string]string{tc.flag: tc.addr},
				`{"attached":true,"reboot_required":true,"rebooted":false}`, "table")
			if err != nil {
				t.Fatalf("second RunE: %v", err)
			}
			if got, ok := body["reboot"].(bool); !ok || got {
				t.Fatalf("reboot = %#v after a prior --reboot run, want false", body["reboot"])
			}
		})
	}
}

// TestServerAttachIPv6RejectsUnattached closes the gap where attach-ipv6
// discarded the response entirely and reported success unconditionally.
func TestServerAttachIPv6RejectsUnattached(t *testing.T) {
	_, _, err := runAttach(t, serverAttachIPv6Cmd,
		map[string]string{"ipv6": "2001:db8::10"},
		`{"attached":false,"reboot_required":false,"rebooted":false}`, "table")
	if err == nil {
		t.Fatal("expected an error when the backend reports attached=false")
	}
	if !strings.Contains(err.Error(), "not attached") {
		t.Fatalf("error = %q, want it to say the IPv6 was not attached", err)
	}
}

func TestServerAttachHonoursOutputFlag(t *testing.T) {
	_, out, err := runAttach(t, serverAttachIPv4Cmd,
		map[string]string{"ipv4": "203.0.113.7"},
		`{"attached":true,"reboot_required":true,"rebooted":false}`, "json")
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var decoded struct {
		Attached       bool `json:"attached"`
		RebootRequired bool `json:"reboot_required"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("-o json output is not valid JSON (%v): %q", err, out)
	}
	if !decoded.Attached || !decoded.RebootRequired {
		t.Fatalf("decoded = %#v, want attached and reboot_required true", decoded)
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
		pidginhost.RESOURCESTATUSENUM_ACTIVE,
		"root",
		false,
		true,
		false,                              // customOs
		false,                              // rescueMode
		*pidginhost.NewNullableString(nil), // bootIso
		false,                              // rescueSupported
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
		"public-ip",
		"new-ipv6",
		"public-ipv6",
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
		publicIP       string
		newIPv6        bool
		publicIPv6     string
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
		publicIP:       serverCreatePublicIP,
		newIPv6:        serverCreateNewIPv6,
		publicIPv6:     serverCreatePublicIPv6,
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
		serverCreatePublicIP = state.publicIP
		serverCreateNewIPv6 = state.newIPv6
		serverCreatePublicIPv6 = state.publicIPv6
		serverCreateNoPubIPv4Ack = state.noPubIPv4Ack
		serverCreatePrivateNetwork = state.privateNetwork
		serverCreatePrivateAddress = state.privateAddress
		serverCreateUserData = state.userData
		serverCreateUserDataFile = state.userDataFile
	}
}
