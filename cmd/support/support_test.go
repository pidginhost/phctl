package support

import (
	"testing"
)

func TestSupportCommandStructure(t *testing.T) {
	if Cmd.Use != "support" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "support")
	}
	for _, a := range Cmd.Aliases {
		if a == "ticket" {
			t.Fatalf("support alias %q should not exist; it collides with the ticket subcommand", a)
		}
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

func TestRootTicketSubcommands(t *testing.T) {
	if TicketCmd.Use != "ticket" {
		t.Fatalf("Use = %q, want %q", TicketCmd.Use, "ticket")
	}

	names := map[string]bool{}
	for _, c := range TicketCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get", "create", "reply", "close", "reopen"} {
		if !names[want] {
			t.Errorf("root ticket missing subcommand %q", want)
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

func TestRootTicketCreateFlags(t *testing.T) {
	createCmd, _, err := TicketCmd.Find([]string{"create"})
	if err != nil {
		t.Fatalf("finding root ticket create command: %v", err)
	}
	for _, flag := range []string{"subject", "department", "message"} {
		f := createCmd.Flags().Lookup(flag)
		if f == nil {
			t.Fatalf("missing --%s flag on root ticket create command", flag)
		}
	}
}

func TestTicketReplyFlags(t *testing.T) {
	f := ticketReplyCmd.Flags().Lookup("message")
	if f == nil {
		t.Fatal("missing --message flag on ticket reply command")
	}
}

func TestRootTicketReplyFlags(t *testing.T) {
	replyCmd, _, err := TicketCmd.Find([]string{"reply", "123"})
	if err != nil {
		t.Fatalf("finding root ticket reply command: %v", err)
	}
	f := replyCmd.Flags().Lookup("message")
	if f == nil {
		t.Fatal("missing --message flag on root ticket reply command")
	}
}
