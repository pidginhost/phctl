package compute

import (
	"testing"
)

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
	for _, name := range []string{"image", "package", "hostname", "project", "ssh-key-id", "password", "new-ipv4"} {
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

func TestPstr(t *testing.T) {
	s := "hello"
	if got := pstr(&s); got != "hello" {
		t.Errorf("pstr(&%q) = %q, want %q", s, got, "hello")
	}
	if got := pstr[string](nil); got != "<none>" {
		t.Errorf("pstr(nil) = %q, want %q", got, "<none>")
	}
}
