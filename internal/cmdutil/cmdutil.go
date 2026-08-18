package cmdutil

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/output"
)

func ParseInt32(s string) (int32, error) {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid ID %q: %w", s, err)
	}
	return int32(n), nil
}

// OutputFormat resolves the global -o/--output flag. Commands exercised
// outside the real root (RunE-level unit tests build bare cobra commands) have
// no such flag, so fall back to the default rather than dereferencing nil.
func OutputFormat(cmd *cobra.Command) output.Format {
	if cmd == nil {
		return output.FormatTable
	}
	flag := cmd.Root().Flag("output")
	if flag == nil {
		return output.FormatTable
	}
	return output.ParseFormat(flag.Value.String())
}

// Force resolves the global -f/--force flag.
//
// It looks the flag up with Flag rather than Flags().GetBool: a root's
// persistent flags are only merged into its Flags() set by Execute, so
// GetBool returned false for anything driven directly through RunE -- which is
// how every RunE-level test runs. A confirmation guard would then look like it
// had not been bypassed no matter what the test set.
func Force(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	flag := cmd.Root().Flag("force")
	if flag == nil {
		return false
	}
	return flag.Value.String() == "true"
}
