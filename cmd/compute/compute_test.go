package compute

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	for _, name := range []string{"image", "package", "hostname", "project", "ssh-key-id", "password", "new-ipv4", "user-data", "user-data-file"} {
		if serverCreateCmd.Flags().Lookup(name) == nil {
			t.Errorf("server create missing flag --%s", name)
		}
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
