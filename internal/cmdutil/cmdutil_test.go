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
