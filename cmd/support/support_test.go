package support

import (
	"bytes"
	"testing"

	pidginhost "github.com/pidginhost/sdk-go"
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

func TestPrintTicketMessagesHandlesEmptyThreads(t *testing.T) {
	for _, messages := range [][]pidginhost.TicketMessage{nil, {}} {
		var out bytes.Buffer
		printTicketMessages(&out, messages)
		if out.Len() != 0 {
			t.Errorf("output = %q, want no message section", out.String())
		}
	}
}

func TestPrintTicketMessagesRendersThreadAndAttachment(t *testing.T) {
	messages := []pidginhost.TicketMessage{
		{
			Date:       "2026-08-17 21:00",
			AuthorName: "Customer",
			Message:    "The server is unavailable.",
		},
		{
			Date:               "2026-08-17 21:05",
			AuthorName:         "Support",
			Message:            "We are checking it.",
			AttachmentFilename: "diagnostic.txt",
		},
	}
	var out bytes.Buffer
	printTicketMessages(&out, messages)
	want := "\nMessages:\n" +
		"\n[2026-08-17 21:00] Customer\nThe server is unavailable.\n" +
		"\n[2026-08-17 21:05] Support\nWe are checking it.\n" +
		"Attachment: diagnostic.txt\n"
	if got := out.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
