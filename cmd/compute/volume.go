package compute

import (
	"context"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var volumeCmd = &cobra.Command{
	Use:     "volume",
	Aliases: []string{"vol"},
	Short:   "Manage storage volumes",
	Args:    cobra.NoArgs,
}

var volumeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudVolumesList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing volumes: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, resp, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "ALIAS", "SIZE", "PRODUCT", "ATTACHED", "SERVER")
			for _, v := range resp {
				output.PrintRow(tw, v.Id, pstr(v.Alias), v.Size, v.Product, v.Attached, v.Server)
			}
			tw.Flush()
		})
	},
}

var volumeGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get volume details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		vol, _, err := c.CloudAPI.CloudVolumesRetrieve(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("getting volume: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, vol, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", vol.Id)
			output.PrintRow(tw, "Alias:", pstr(vol.Alias))
			output.PrintRow(tw, "Size:", vol.Size)
			output.PrintRow(tw, "Product:", vol.Product)
			output.PrintRow(tw, "Attached:", vol.Attached)
			output.PrintRow(tw, "Server:", vol.Server)
			tw.Flush()
		})
	},
}

var volumeDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a volume",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete volume %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudVolumesDestroy(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("deleting volume: %w", err)
		}
		fmt.Printf("Volume %d deleted.\n", id)
		return nil
	},
}

var volumeAttachVM int32

var volumeAttachCmd = &cobra.Command{
	Use:   "attach <volume-id>",
	Short: "Attach a volume to a server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewAttachVolume(volumeAttachVM)
		_, _, err = c.CloudAPI.CloudVolumesAttachCreate(context.Background(), id).AttachVolume(body).Execute()
		if err != nil {
			return fmt.Errorf("attaching volume: %w", err)
		}
		fmt.Printf("Volume %d attached to server %d.\n", id, volumeAttachVM)
		return nil
	},
}

var volumeDetachCmd = &cobra.Command{
	Use:   "detach <volume-id>",
	Short: "Detach a volume from its server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudVolumesDetachCreate(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("detaching volume: %w", err)
		}
		fmt.Printf("Volume detached: %v\n", resp.Detached)
		return nil
	},
}

func init() {
	volumeAttachCmd.Flags().Int32Var(&volumeAttachVM, "server", 0, "Server ID to attach to (required)")
	volumeAttachCmd.MarkFlagRequired("server")

	volumeCmd.AddCommand(volumeListCmd)
	volumeCmd.AddCommand(volumeGetCmd)
	volumeCmd.AddCommand(volumeDeleteCmd)
	volumeCmd.AddCommand(volumeAttachCmd)
	volumeCmd.AddCommand(volumeDetachCmd)
}
