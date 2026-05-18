package compute

import (
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
		generation, err := cmd.Flags().GetString("generation")
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		packages, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.ServerProduct, bool, error) {
			req := c.CloudAPI.CloudServerPackagesList(cmd.Context()).Page(page)
			if generation != "" {
				req = req.Generation(generation)
			}
			resp, _, err := req.Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing packages", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, packages, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "SLUG", "CPUS", "MEMORY_GB", "DISK_GB", "TRAFFIC_GB")
			for _, p := range packages {
				output.PrintRow(tw, p.Id, p.Name, p.Slug, p.Cpus, p.Memory, p.DiskSize, p.Traffic)
			}
			tw.Flush()
		})
	},
}

func init() {
	packageListCmd.Flags().String("generation", "", "Filter packages available on the given hardware generation (slug)")

	packageCmd.AddCommand(packageListCmd)
}
