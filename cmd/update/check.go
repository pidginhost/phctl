package update

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	iupdate "github.com/pidginhost/phctl/internal/update"
)

var CheckCmd = &cobra.Command{
	Use:    "__update-check <current-version>",
	Short:  "Run the internal update availability check",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if notice := iupdate.CheckNotice(args[0]); notice != "" {
			if _, err := fmt.Fprint(os.Stderr, notice); err != nil {
				return err
			}
		}
		return nil
	},
}
