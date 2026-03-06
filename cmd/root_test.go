package cmd

import (
	"testing"
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
	}
	if output.Shorthand != "o" {
		t.Errorf("output shorthand = %q, want %q", output.Shorthand, "o")
	}

	force := rootCmd.PersistentFlags().Lookup("force")
	if force == nil {
		t.Fatal("force flag not registered")
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

	for _, want := range []string{"auth", "compute", "account", "domain", "kubernetes", "billing", "dedicated", "freedns", "hosting", "support"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestSetVersion(t *testing.T) {
	SetVersion("1.2.3")
	if rootCmd.Version != "1.2.3" {
		t.Errorf("version = %q, want %q", rootCmd.Version, "1.2.3")
	}
	SetVersion("")
}
