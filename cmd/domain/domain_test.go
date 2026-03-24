package domain

import (
	"testing"
)

func TestDomainCommandStructure(t *testing.T) {
	if Cmd.Use != "domain" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "domain")
	}

	aliases := Cmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "dns" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("domain Aliases = %v, want to contain 'dns'", aliases)
	}
}

func TestDomainSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "get", "create", "check", "renew", "cancel", "transfer", "nameservers", "tld", "registrant"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestDomainCreateFlags(t *testing.T) {
	ns := domainCreateCmd.Flags().Lookup("nameservers")
	if ns == nil {
		t.Fatal("missing --nameservers flag")
	}
	years := domainCreateCmd.Flags().Lookup("years")
	if years == nil {
		t.Fatal("missing --years flag")
	}
}

func TestDomainRenewFlags(t *testing.T) {
	years := domainRenewCmd.Flags().Lookup("years")
	if years == nil {
		t.Fatal("missing --years flag on renew")
		return
	}
	if years.DefValue != "1" {
		t.Errorf("renew --years default = %q, want %q", years.DefValue, "1")
	}
}

func TestTLDSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range tldCmd.Commands() {
		names[c.Name()] = true
	}
	if !names["list"] {
		t.Error("tld missing subcommand 'list'")
	}
}

func TestRegistrantSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range registrantCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "get", "create", "delete"} {
		if !names[want] {
			t.Errorf("registrant missing subcommand %q", want)
		}
	}
}

func TestRegistrantCreateFlags(t *testing.T) {
	required := []string{"first-name", "last-name", "email", "phone", "address", "city", "region", "postal-code", "country"}
	for _, name := range required {
		f := registrantCreateCmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("registrant create missing flag --%s", name)
		}
	}
}

func TestRegistrantDeleteAliases(t *testing.T) {
	aliases := registrantDeleteCmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "rm" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("registrant delete Aliases = %v, want to contain 'rm'", aliases)
	}
}
