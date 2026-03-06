package compute

import (
	"context"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/output"
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "List available OS images",
	Args:  cobra.NoArgs,
}

var imageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available images",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		images, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.OSImage, bool, error) {
			resp, _, err := c.CloudAPI.CloudImagesList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing images: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, images, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "SLUG")
			for _, img := range images {
				output.PrintRow(tw, img.Id, img.Name, img.Slug)
			}
			tw.Flush()
		})
	},
}

func init() {
	imageCmd.AddCommand(imageListCmd)
}
