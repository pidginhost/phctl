package account

import (
	"testing"
)

func TestAccountCommandStructure(t *testing.T) {
	if Cmd.Use != "account" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "account")
	}
}

func TestAccountSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"profile", "ssh-key", "company", "api-token", "email"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestSSHKeySubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range sshKeyCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "create", "delete"} {
		if !names[want] {
			t.Errorf("ssh-key missing subcommand %q", want)
		}
	}
}

func TestSSHKeyAliases(t *testing.T) {
	aliases := sshKeyCmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "ssh" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ssh-key Aliases = %v, want to contain 'ssh'", aliases)
	}
}

func TestSSHKeyDeleteAliases(t *testing.T) {
	aliases := sshKeyDeleteCmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "rm" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ssh-key delete Aliases = %v, want to contain 'rm'", aliases)
	}
}

func TestCompanySubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range companyCmd.Commands() {
		names[c.Name()] = true
	}

	if !names["list"] {
		t.Error("company missing subcommand 'list'")
	}
}

func TestSSHKeyCreateFlags(t *testing.T) {
	keyFlag := sshKeyCreateCmd.Flags().Lookup("key")
	if keyFlag == nil {
		t.Fatal("missing --key flag on ssh-key create")
	}

	aliasFlag := sshKeyCreateCmd.Flags().Lookup("alias")
	if aliasFlag == nil {
		t.Fatal("missing --alias flag on ssh-key create")
	}
}
