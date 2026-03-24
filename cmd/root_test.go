package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandStructure(t *testing.T) {
	if rootCmd.Use != "phctl" {
		t.Errorf("root Use = %q, want %q", rootCmd.Use, "phctl")
	}
	if !rootCmd.SilenceErrors {
		t.Error("SilenceErrors should be true")
	}
	if !rootCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestRootPersistentFlags(t *testing.T) {
	output := rootCmd.PersistentFlags().Lookup("output")
	if output == nil {
		t.Fatal("output flag not registered")
		return
	}
	if output.Shorthand != "o" {
		t.Errorf("output shorthand = %q, want %q", output.Shorthand, "o")
	}

	force := rootCmd.PersistentFlags().Lookup("force")
	if force == nil {
		t.Fatal("force flag not registered")
		return
	}
	if force.Shorthand != "f" {
		t.Errorf("force shorthand = %q, want %q", force.Shorthand, "f")
	}
}

func TestRootSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"auth", "compute", "account", "domain", "kubernetes", "billing", "dedicated", "freedns", "hosting", "support", "ticket", "update"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestTicketCommandRoutes(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"ticket", "list"}, want: "phctl ticket list"},
		{args: []string{"support", "ticket", "list"}, want: "phctl support ticket list"},
	}

	for _, tt := range tests {
		cmd, _, err := rootCmd.Find(tt.args)
		if err != nil {
			t.Fatalf("Find(%v) error: %v", tt.args, err)
		}
		if got := cmd.CommandPath(); got != tt.want {
			t.Fatalf("Find(%v) command path = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestFlagOnlyLeafCommandsRejectExtraArgs(t *testing.T) {
	cmd, args, err := rootCmd.Find([]string{"auth", "login", "extra"})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if err := cmd.Args(cmd, args); err == nil {
		t.Fatal("expected auth login to reject extra positional arguments")
	}
}

func TestAllCommandsDefineArgs(t *testing.T) {
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		switch cmd.Name() {
		case "help", "completion", "__complete", "__completeNoDesc":
			return
		}
		if cmd.Args == nil {
			t.Errorf("%s is missing an Args validator", cmd.CommandPath())
		}
	})
}

func TestValidateOutputFlag(t *testing.T) {
	orig := rootCmd.PersistentFlags().Lookup("output").Value.String()
	t.Cleanup(func() {
		if err := rootCmd.PersistentFlags().Set("output", orig); err != nil {
			t.Fatalf("restoring output flag: %v", err)
		}
	})

	if err := rootCmd.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatalf("setting output flag: %v", err)
	}
	if err := validateOutputFlag(rootCmd); err != nil {
		t.Fatalf("validateOutputFlag(json) error: %v", err)
	}

	if err := rootCmd.PersistentFlags().Set("output", "csv"); err != nil {
		t.Fatalf("setting output flag: %v", err)
	}
	if err := validateOutputFlag(rootCmd); err == nil {
		t.Fatal("validateOutputFlag(csv) unexpectedly succeeded")
	}
}

func TestSetVersion(t *testing.T) {
	SetVersion("1.2.3")
	if rootCmd.Version != "1.2.3" {
		t.Errorf("version = %q, want %q", rootCmd.Version, "1.2.3")
	}
	SetVersion("")
}

func walkCommands(cmd *cobra.Command, visit func(*cobra.Command)) {
	visit(cmd)
	for _, child := range cmd.Commands() {
		walkCommands(child, visit)
	}
}
