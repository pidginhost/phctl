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

func Force(cmd *cobra.Command) bool {
	f, _ := cmd.Root().Flags().GetBool("force")
	return f
}
