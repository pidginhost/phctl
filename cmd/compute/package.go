package compute

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/output"
)

var packageCmd = &cobra.Command{
	Use:     "package",
	Aliases: []string{"pkg"},
	Short:   "List available server packages",
}

var packageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all server packages",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudServerPackagesList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing packages: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, resp.Results, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "SLUG")
			for _, p := range resp.Results {
				output.PrintRow(tw, p.Id, p.Name, p.Slug)
			}
			tw.Flush()
		})
		return nil
	},
}

func init() {
	packageCmd.AddCommand(packageListCmd)
}
