package support

import (
	"testing"
)

func TestSupportCommandStructure(t *testing.T) {
	if Cmd.Use != "support" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "support")
	}
	found := false
	for _, a := range Cmd.Aliases {
		if a == "ticket" {
			found = true
		}
	}
	if !found {
		t.Errorf("Aliases = %v, want to contain 'ticket'", Cmd.Aliases)
	}
}

func TestSupportSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"department", "ticket"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestDepartmentSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range departmentCmd.Commands() {
		names[c.Name()] = true
	}
	if !names["list"] {
		t.Error("department missing subcommand 'list'")
	}
}

func TestTicketSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range ticketCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get", "create", "reply", "close", "reopen"} {
		if !names[want] {
			t.Errorf("ticket missing subcommand %q", want)
		}
	}
}

func TestTicketCreateFlags(t *testing.T) {
	for _, flag := range []string{"subject", "department", "message"} {
		f := ticketCreateCmd.Flags().Lookup(flag)
		if f == nil {
			t.Fatalf("missing --%s flag on ticket create command", flag)
		}
	}
}

func TestTicketReplyFlags(t *testing.T) {
	f := ticketReplyCmd.Flags().Lookup("message")
	if f == nil {
		t.Fatal("missing --message flag on ticket reply command")
	}
}
