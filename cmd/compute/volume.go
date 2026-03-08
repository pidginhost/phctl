package compute

import (
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
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
		resp, _, err := c.CloudAPI.CloudVolumesList(cmd.Context()).Execute()
		if err != nil {
			return fmt.Errorf("listing volumes: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, resp, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "ALIAS", "SIZE", "PRODUCT", "ATTACHED", "SERVER")
			for _, v := range resp {
				output.PrintRow(tw, v.Id, output.Pstr(v.Alias), v.Size, v.Product, v.Attached, v.Server)
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
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		vol, _, err := c.CloudAPI.CloudVolumesRetrieve(cmd.Context(), id).Execute()
		if err != nil {
			return fmt.Errorf("getting volume: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, vol, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", vol.Id)
			output.PrintRow(tw, "Alias:", output.Pstr(vol.Alias))
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
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete volume %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudVolumesDestroy(cmd.Context(), id).Execute()
		if err != nil {
			return fmt.Errorf("deleting volume: %w", err)
		}
		cmd.Printf("Volume %d deleted.\n", id)
		return nil
	},
}

var volumeAttachVM int32

var volumeAttachCmd = &cobra.Command{
	Use:   "attach <volume-id>",
	Short: "Attach a volume to a server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewAttachVolume(volumeAttachVM)
		_, _, err = c.CloudAPI.CloudVolumesAttachCreate(cmd.Context(), id).AttachVolume(body).Execute()
		if err != nil {
			return fmt.Errorf("attaching volume: %w", err)
		}
		cmd.Printf("Volume %d attached to server %d.\n", id, volumeAttachVM)
		return nil
	},
}

var volumeDetachCmd = &cobra.Command{
	Use:   "detach <volume-id>",
	Short: "Detach a volume from its server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudVolumesDetachCreate(cmd.Context(), id).Execute()
		if err != nil {
			return fmt.Errorf("detaching volume: %w", err)
		}
		cmd.Printf("Volume detached: %v\n", resp.Detached)
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
