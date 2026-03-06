package freedns

import (
	"testing"
)

func TestFreeDNSCommandStructure(t *testing.T) {
	if Cmd.Use != "freedns" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "freedns")
	}
	found := false
	for _, a := range Cmd.Aliases {
		if a == "fdns" {
			found = true
		}
	}
	if !found {
		t.Errorf("Aliases = %v, want to contain 'fdns'", Cmd.Aliases)
	}
}

func TestFreeDNSSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"domain", "record"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestDomainSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range domainCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "activate", "deactivate"} {
		if !names[want] {
			t.Errorf("domain missing subcommand %q", want)
		}
	}
}

func TestRecordSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range recordCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "create", "delete"} {
		if !names[want] {
			t.Errorf("record missing subcommand %q", want)
		}
	}
}

func TestActivateFlags(t *testing.T) {
	f := domainActivateCmd.Flags().Lookup("ip")
	if f == nil {
		t.Fatal("missing --ip flag on activate command")
	}
}

func TestRecordCreateFlags(t *testing.T) {
	for _, flag := range []string{"domain", "type", "name"} {
		f := recordCreateCmd.Flags().Lookup(flag)
		if f == nil {
			t.Fatalf("missing --%s flag on record create command", flag)
		}
	}
}
