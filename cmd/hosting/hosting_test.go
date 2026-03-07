package hosting

import (
	"testing"
)

func TestHostingCommandStructure(t *testing.T) {
	if Cmd.Use != "hosting" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "hosting")
	}
	found := false
	for _, a := range Cmd.Aliases {
		if a == "host" {
			found = true
		}
	}
	if !found {
		t.Errorf("Aliases = %v, want to contain 'host'", Cmd.Aliases)
	}
}

func TestHostingSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"service", "change-password"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestServiceSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range serviceCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get"} {
		if !names[want] {
			t.Errorf("service missing subcommand %q", want)
		}
	}
}

func TestChangePasswordFlags(t *testing.T) {
	f := changePasswordCmd.Flags().Lookup("password")
	if f == nil {
		t.Fatal("missing --password flag on change-password command")
	}
}
