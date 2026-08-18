package cmdutil

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestParseInt32(t *testing.T) {
	tests := []struct {
		input   string
		want    int32
		wantErr bool
	}{
		{"1", 1, false},
		{"0", 0, false},
		{"2147483647", 2147483647, false},
		{"-1", -1, false},
		{"abc", 0, true},
		{"", 0, true},
		{"99999999999", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseInt32(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseInt32(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseInt32(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().StringP("output", "o", "table", "Output format")
	root.PersistentFlags().BoolP("force", "f", false, "Skip confirmation")
	child := &cobra.Command{Use: "sub"}
	root.AddCommand(child)
	return root
}

func TestOutputFormat(t *testing.T) {
	root := newRootCmd()
	child := root.Commands()[0]

	// Default
	got := OutputFormat(child)
	if got != "table" {
		t.Errorf("OutputFormat default = %q, want %q", got, "table")
	}

	// Set to json
	if err := root.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatalf("setting output flag: %v", err)
	}
	got = OutputFormat(child)
	if got != "json" {
		t.Errorf("OutputFormat json = %q, want %q", got, "json")
	}
}

// TestOutputFormatWithoutRootFlag pins the fallback for commands that are not
// wired under the real root: RunE-level unit tests construct bare cobra
// commands, and looking up a flag that does not exist there must not panic.
func TestOutputFormatWithoutRootFlag(t *testing.T) {
	if got := OutputFormat(&cobra.Command{Use: "orphan"}); got != "table" {
		t.Errorf("OutputFormat without --output = %q, want %q", got, "table")
	}
}

func TestForce(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().BoolP("force", "f", false, "Skip confirmation")
	child := &cobra.Command{Use: "sub", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	root.AddCommand(child)

	// Default: false
	root.SetArgs([]string{"sub"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if Force(child) {
		t.Error("Force default should be false")
	}

	// With --force
	root.SetArgs([]string{"sub", "--force"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !Force(child) {
		t.Error("Force should be true with --force flag")
	}
}

// TestForce is written around root.Execute(), which is what merges a root's
// persistent flags into its Flags() set. RunE-level unit tests never call
// Execute, so Force silently returned false there and any confirmation guard
// looked like it had not been bypassed. Resolve the flag the same way
// OutputFormat does, which handles persistent flags directly.
func TestForceReadsPersistentFlagWithoutExecute(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().BoolP("force", "f", true, "Skip confirmation")
	child := &cobra.Command{Use: "sub"}
	root.AddCommand(child)

	if !Force(child) {
		t.Error("Force should see --force set on the root's persistent flags before Execute")
	}
}

func TestForceOnCommandWithoutForceFlag(t *testing.T) {
	child := &cobra.Command{Use: "sub"}
	if Force(child) {
		t.Error("Force should default to false when no --force flag exists")
	}
	if Force(nil) {
		t.Error("Force(nil) should be false, not panic")
	}
}
