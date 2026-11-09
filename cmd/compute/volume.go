package compute

import (
	"fmt"
	"io"
	"strconv"

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
			return cmdutil.APIError("listing volumes", err)
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
			return cmdutil.APIError("getting volume", err)
		}
		if vol == nil {
			return fmt.Errorf("getting volume: server returned no response")
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

var volumeCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a volume and attach it to a server",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()
		server, err := flags.GetInt32("server")
		if err != nil {
			return err
		}
		size, err := flags.GetInt32("size")
		if err != nil {
			return err
		}
		product, err := flags.GetString("product")
		if err != nil {
			return err
		}
		alias, err := flags.GetString("alias")
		if err != nil {
			return err
		}
		project, err := flags.GetString("project")
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := pidginhost.NewVolumeRequest(size, product)
		if alias != "" {
			body.SetAlias(alias)
		}
		if project != "" {
			body.SetProject(project)
		}
		vol, _, err := c.CloudAPI.CloudServersVolumesCreate(cmd.Context(), strconv.Itoa(int(server))).
			VolumeRequest(*body).Execute()
		if err != nil {
			return cmdutil.APIError("creating volume", err)
		}
		if vol == nil {
			return fmt.Errorf("creating volume: server returned no response")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), vol,
			"Volume %d created on server %d (%d GB).\n", vol.Id, server, vol.Size)
	},
}

var volumeProductsCmd = &cobra.Command{
	Use:   "products",
	Short: "List the volume types you can create",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		products, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.StorageProduct, bool, error) {
			resp, _, err := c.CloudAPI.CloudStorageProductsList(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			if resp == nil {
				return nil, false, fmt.Errorf("server returned no response page")
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing volume products", err)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), products, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "SLUG", "PRICE", "UNIT", "MIN_SIZE", "MAX_SIZE")
			for _, p := range products {
				output.PrintRow(tw, p.Id, p.Name, p.Slug, p.Price, p.Unit, p.MinSize, p.MaxSize)
			}
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
			return cmdutil.APIError("deleting volume", err)
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
		body := *pidginhost.NewAttachVolumeRequest(volumeAttachVM)
		resp, _, err := c.CloudAPI.CloudVolumesAttachCreate(cmd.Context(), id).AttachVolumeRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("attaching volume", err)
		}
		if resp == nil {
			return fmt.Errorf("attaching volume %d: server returned no result", id)
		}
		if !resp.Attached {
			return fmt.Errorf("attaching volume %d: server reported it was not attached to server %d", id, volumeAttachVM)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"Volume %d attached to server %d.\n", id, volumeAttachVM)
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
			return cmdutil.APIError("detaching volume", err)
		}
		if resp == nil {
			return fmt.Errorf("detaching volume: server returned no response")
		}
		if !resp.Detached {
			return fmt.Errorf("detaching volume: server did not detach the volume")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"Volume detached: %v\n", resp.Detached)
	},
}

func init() {
	volumeAttachCmd.Flags().Int32Var(&volumeAttachVM, "server", 0, "Server ID to attach to (required)")
	volumeAttachCmd.MarkFlagRequired("server")

	volumeCreateCmd.Flags().Int32("server", 0, "Server ID to attach the new volume to (required)")
	volumeCreateCmd.Flags().Int32("size", 0, "Size in GB (required)")
	volumeCreateCmd.Flags().String("product", "", "Volume product ID or slug, see 'volume products' (required)")
	volumeCreateCmd.Flags().String("alias", "", "Volume name")
	volumeCreateCmd.Flags().String("project", "", "Project name")
	for _, name := range []string{"server", "size", "product"} {
		volumeCreateCmd.MarkFlagRequired(name)
	}

	volumeCmd.AddCommand(volumeListCmd)
	volumeCmd.AddCommand(volumeGetCmd)
	volumeCmd.AddCommand(volumeCreateCmd)
	volumeCmd.AddCommand(volumeProductsCmd)
	volumeCmd.AddCommand(volumeDeleteCmd)
	volumeCmd.AddCommand(volumeAttachCmd)
	volumeCmd.AddCommand(volumeDetachCmd)
}
