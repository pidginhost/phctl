package compute

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/output"
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "List available OS images",
}

var imageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available images",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudImagesList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing images: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, resp.Results, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "SLUG")
			for _, img := range resp.Results {
				output.PrintRow(tw, img.Id, img.Name, img.Slug)
			}
			tw.Flush()
		})
		return nil
	},
}

func init() {
	imageCmd.AddCommand(imageListCmd)
}
