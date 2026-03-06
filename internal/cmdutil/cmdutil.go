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

func OutputFormat(cmd *cobra.Command) output.Format {
	return output.ParseFormat(cmd.Root().Flag("output").Value.String())
}

func Force(cmd *cobra.Command) bool {
	f, _ := cmd.Root().Flags().GetBool("force")
	return f
}
