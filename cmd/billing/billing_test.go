package billing

import (
	"testing"
)

func TestBillingCommandStructure(t *testing.T) {
	if Cmd.Use != "billing" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "billing")
	}
	found := false
	for _, a := range Cmd.Aliases {
		if a == "bill" {
			found = true
		}
	}
	if !found {
		t.Errorf("Aliases = %v, want to contain 'bill'", Cmd.Aliases)
	}
}

func TestBillingSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"funds", "deposit", "invoice", "service", "subscription"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestFundsSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range fundsCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"balance", "log"} {
		if !names[want] {
			t.Errorf("funds missing subcommand %q", want)
		}
	}
}

func TestDepositSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range depositCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get", "create"} {
		if !names[want] {
			t.Errorf("deposit missing subcommand %q", want)
		}
	}
}

func TestInvoiceSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range invoiceCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get", "pay"} {
		if !names[want] {
			t.Errorf("invoice missing subcommand %q", want)
		}
	}
}

func TestServiceSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range serviceCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get", "cancel", "toggle-auto-pay"} {
		if !names[want] {
			t.Errorf("service missing subcommand %q", want)
		}
	}
}

func TestSubscriptionSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range subscriptionCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get"} {
		if !names[want] {
			t.Errorf("subscription missing subcommand %q", want)
		}
	}
}

func TestDepositCreateFlags(t *testing.T) {
	f := depositCreateCmd.Flags().Lookup("amount")
	if f == nil {
		t.Fatal("missing --amount flag on deposit create")
	}
}
