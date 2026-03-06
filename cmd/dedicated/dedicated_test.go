package dedicated

import (
	"testing"
)

func TestDedicatedCommandStructure(t *testing.T) {
	if Cmd.Use != "dedicated" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "dedicated")
	}
	found := false
	for _, a := range Cmd.Aliases {
		if a == "ded" {
			found = true
		}
	}
	if !found {
		t.Errorf("Aliases = %v, want to contain 'ded'", Cmd.Aliases)
	}
}

func TestDedicatedSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}
	if !names["server"] {
		t.Error("missing subcommand 'server'")
	}
}

func TestServerSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range serverCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get", "power", "reinstall", "rdns"} {
		if !names[want] {
			t.Errorf("server missing subcommand %q", want)
		}
	}
}

func TestPowerFlags(t *testing.T) {
	f := serverPowerCmd.Flags().Lookup("action")
	if f == nil {
		t.Fatal("missing --action flag on power command")
	}
}

func TestReinstallFlags(t *testing.T) {
	f := serverReinstallCmd.Flags().Lookup("os-id")
	if f == nil {
		t.Fatal("missing --os-id flag on reinstall command")
	}
}

func TestRDNSFlags(t *testing.T) {
	for _, flag := range []string{"ip-id", "hostname"} {
		f := serverRDNSCmd.Flags().Lookup(flag)
		if f == nil {
			t.Fatalf("missing --%s flag on rdns command", flag)
		}
	}
}
