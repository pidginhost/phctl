package compute

import (
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/output"
)

var packageCmd = &cobra.Command{
	Use:     "package",
	Aliases: []string{"pkg"},
	Short:   "List available server packages",
	Args:    cobra.NoArgs,
}

var packageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all server packages",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		packages, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.ServerProduct, bool, error) {
			resp, _, err := c.CloudAPI.CloudServerPackagesList(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing packages: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, packages, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "SLUG")
			for _, p := range packages {
				output.PrintRow(tw, p.Id, p.Name, p.Slug)
			}
			tw.Flush()
		})
	},
}

func init() {
	packageCmd.AddCommand(packageListCmd)
}
