package freedns

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

func TestRecordCommandsRequireSource(t *testing.T) {
	for _, command := range []*cobra.Command{recordListCmd, recordCreateCmd, recordDeleteCmd} {
		t.Run(command.Name(), func(t *testing.T) {
			// Supply every other required flag without making an API call.
			command.Flags().VisitAll(func(flag *pflag.Flag) {
				changed := flag.Changed
				t.Cleanup(func() { flag.Changed = changed })
				flag.Changed = flag.Name != "source"
			})
			if err := command.ValidateRequiredFlags(); err == nil || !strings.Contains(err.Error(), "source") {
				t.Fatalf("missing --source: error = %v, want required flag error", err)
			}
			command.Flags().Lookup("source").Changed = true
			if err := command.ValidateRequiredFlags(); err != nil {
				t.Fatalf("supplied --source: %v", err)
			}
		})
	}
}
