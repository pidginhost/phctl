package compute

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var serverCmd = &cobra.Command{
	Use:     "server",
	Aliases: []string{"s"},
	Short:   "Manage cloud servers",
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		servers, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.Server, bool, error) {
			resp, _, err := c.CloudAPI.CloudServersList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing servers: %w", err)
		}
		return output.Print(outputFormat(cmd), servers, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME", "IMAGE", "PACKAGE", "STATUS")
			for _, s := range servers {
				output.PrintRow(tw, s.Id, pstr(s.Hostname), s.Image, s.Package, pstr(s.Status))
			}
			tw.Flush()
		})
	},
}

var serverGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get server details",
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
		httpResp, err := c.CloudAPI.CloudServersRetrieve(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("getting server: %w", err)
		}
		defer httpResp.Body.Close()

		var server pidginhost.Server
		if err := json.NewDecoder(httpResp.Body).Decode(&server); err != nil {
			return fmt.Errorf("decoding server: %w", err)
		}

		return output.Print(outputFormat(cmd), server, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", server.Id)
			output.PrintRow(tw, "Hostname:", pstr(server.Hostname))
			output.PrintRow(tw, "Image:", server.Image)
			output.PrintRow(tw, "Package:", server.Package)
			output.PrintRow(tw, "CPUs:", server.Cpus)
			output.PrintRow(tw, "Memory:", server.Memory)
			output.PrintRow(tw, "Disk:", server.DiskSize)
			output.PrintRow(tw, "Status:", pstr(server.Status))
			output.PrintRow(tw, "Destroy Protection:", server.DestroyProtection)
			output.PrintRow(tw, "HA Enabled:", server.HaEnabled)
			tw.Flush()
		})
	},
}

var (
	serverCreateImage    string
	serverCreatePackage  string
	serverCreateHostname string
	serverCreateProject  string
	serverCreateSSHKeyID string
	serverCreatePassword string
	serverCreateNewIPv4  bool
)

var serverCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new server",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}

		body := *pidginhost.NewServerAdd(serverCreateImage, serverCreatePackage)
		if serverCreateHostname != "" {
			body.Hostname = pidginhost.PtrString(serverCreateHostname)
		}
		if serverCreateProject != "" {
			body.Project = pidginhost.PtrString(serverCreateProject)
		}
		if serverCreateSSHKeyID != "" {
			body.SshPubKeyId = pidginhost.PtrString(serverCreateSSHKeyID)
		}
		if serverCreatePassword != "" {
			body.Password = pidginhost.PtrString(serverCreatePassword)
		}
		if serverCreateNewIPv4 {
			body.NewIpv4 = pidginhost.PtrBool(true)
		}

		resp, _, err := c.CloudAPI.CloudServersCreate(context.Background()).ServerAdd(body).Execute()
		if err != nil {
			return fmt.Errorf("creating server: %w", err)
		}

		fmt.Printf("Server created (ID: %d)\n", resp.Id)
		return nil
	},
}

var serverDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a server",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete server %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudServersDestroy(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("deleting server: %w", err)
		}
		fmt.Printf("Server %d deleted.\n", id)
		return nil
	},
}

var serverPowerAction string

var serverPowerCmd = &cobra.Command{
	Use:   "power <id>",
	Short: "Manage server power (--action start|stop|shutdown|reboot)",
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
		body := *pidginhost.NewPowerManagementRequest(pidginhost.PowerManagementRequestActionEnum(serverPowerAction))
		_, _, err = c.CloudAPI.CloudServersPowerManagementCreate(context.Background(), id).PowerManagementRequest(body).Execute()
		if err != nil {
			return fmt.Errorf("power management: %w", err)
		}
		fmt.Printf("Power action '%s' executed on server %d.\n", serverPowerAction, id)
		return nil
	},
}

var serverConsoleCmd = &cobra.Command{
	Use:   "console <id>",
	Short: "Get server console token",
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
		resp, _, err := c.CloudAPI.CloudServersConsoleCreate(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("getting console: %w", err)
		}
		return output.Print(outputFormat(cmd), resp, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "Token:", resp.Token)
			output.PrintRow(tw, "Ticket:", resp.Ticket)
			tw.Flush()
		})
	},
}

// --- Attach / Detach IP ---

var serverAttachIPv4Slug string

var serverAttachIPv4Cmd = &cobra.Command{
	Use:   "attach-ipv4 <server-id>",
	Short: "Attach an IPv4 address to a server",
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
		body := *pidginhost.NewAttachIPv4(serverAttachIPv4Slug)
		_, _, err = c.CloudAPI.CloudServersAttachIpv4Create(context.Background(), id).AttachIPv4(body).Execute()
		if err != nil {
			return fmt.Errorf("attaching IPv4: %w", err)
		}
		fmt.Printf("IPv4 %s attached to server %d.\n", serverAttachIPv4Slug, id)
		return nil
	},
}

var serverAttachIPv6Slug string

var serverAttachIPv6Cmd = &cobra.Command{
	Use:   "attach-ipv6 <server-id>",
	Short: "Attach an IPv6 address to a server",
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
		body := *pidginhost.NewAttachIPv6(serverAttachIPv6Slug)
		_, _, err = c.CloudAPI.CloudServersAttachIpv6Create(context.Background(), id).AttachIPv6(body).Execute()
		if err != nil {
			return fmt.Errorf("attaching IPv6: %w", err)
		}
		fmt.Printf("IPv6 %s attached to server %d.\n", serverAttachIPv6Slug, id)
		return nil
	},
}

// --- Destroy protection ---

var serverProtectEnable bool

var serverProtectCmd = &cobra.Command{
	Use:   "protect <id>",
	Short: "Toggle destroy protection on a server",
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
		body := *pidginhost.NewDestroyProtection(serverProtectEnable)
		_, _, err = c.CloudAPI.CloudServersDestroyProtectionCreate(context.Background(), id).DestroyProtection(body).Execute()
		if err != nil {
			return fmt.Errorf("setting destroy protection: %w", err)
		}
		state := "enabled"
		if !serverProtectEnable {
			state = "disabled"
		}
		fmt.Printf("Destroy protection %s on server %d.\n", state, id)
		return nil
	},
}

// --- Snapshots ---

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage server snapshots",
}

var serverSnapshotListCmd = &cobra.Command{
	Use:   "list <server-id>",
	Short: "List snapshots for a server",
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
		snapshots, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.Snapshot, bool, error) {
			resp, _, err := c.CloudAPI.CloudServersSnapshotsList(context.Background(), id).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing snapshots: %w", err)
		}
		return output.Print(outputFormat(cmd), snapshots, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "NAME", "STATE", "CREATED")
			for _, s := range snapshots {
				created := "<none>"
				if t := s.CreatedAt.Get(); t != nil {
					created = t.Format("2006-01-02 15:04:05")
				}
				output.PrintRow(tw, s.Name, s.State, created)
			}
			tw.Flush()
		})
	},
}

var snapshotCreateName string

var serverSnapshotCreateCmd = &cobra.Command{
	Use:   "create <server-id>",
	Short: "Create a snapshot for a server",
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
		body := *pidginhost.NewSnapshotCreate(snapshotCreateName)
		_, _, err = c.CloudAPI.CloudServersSnapshotsCreate(context.Background(), id).SnapshotCreate(body).Execute()
		if err != nil {
			return fmt.Errorf("creating snapshot: %w", err)
		}
		fmt.Printf("Snapshot '%s' creation queued.\n", snapshotCreateName)
		return nil
	},
}

var serverSnapshotDeleteCmd = &cobra.Command{
	Use:     "delete <server-id> <snapshot-name>",
	Aliases: []string{"rm"},
	Short:   "Delete a snapshot",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete snapshot '%s' from server %d?", args[1], id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, _, err = c.CloudAPI.CloudServersSnapshotsDestroy(context.Background(), id, args[1]).Execute()
		if err != nil {
			return fmt.Errorf("deleting snapshot: %w", err)
		}
		fmt.Printf("Snapshot '%s' deletion queued.\n", args[1])
		return nil
	},
}

var serverSnapshotRollbackCmd = &cobra.Command{
	Use:   "rollback <server-id> <snapshot-name>",
	Short: "Rollback a server to a snapshot",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Rollback server %d to snapshot '%s'?", id, args[1])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, _, err = c.CloudAPI.CloudServersSnapshotsRollbackCreate(context.Background(), id, args[1]).Execute()
		if err != nil {
			return fmt.Errorf("rolling back snapshot: %w", err)
		}
		fmt.Printf("Rollback to snapshot '%s' queued.\n", args[1])
		return nil
	},
}

func init() {
	serverCreateCmd.Flags().StringVar(&serverCreateImage, "image", "", "OS image (required)")
	serverCreateCmd.Flags().StringVar(&serverCreatePackage, "package", "", "Server package (required)")
	serverCreateCmd.Flags().StringVar(&serverCreateHostname, "hostname", "", "Server hostname")
	serverCreateCmd.Flags().StringVar(&serverCreateProject, "project", "", "Project name")
	serverCreateCmd.Flags().StringVar(&serverCreateSSHKeyID, "ssh-key-id", "", "SSH key ID to inject")
	serverCreateCmd.Flags().StringVar(&serverCreatePassword, "password", "", "Root password")
	serverCreateCmd.Flags().BoolVar(&serverCreateNewIPv4, "new-ipv4", false, "Allocate a new public IPv4")
	serverCreateCmd.MarkFlagRequired("image")
	serverCreateCmd.MarkFlagRequired("package")

	serverPowerCmd.Flags().StringVar(&serverPowerAction, "action", "", "Power action: start, stop, shutdown, reboot")
	serverPowerCmd.MarkFlagRequired("action")

	serverAttachIPv4Cmd.Flags().StringVar(&serverAttachIPv4Slug, "ipv4", "", "IPv4 ID or slug (required)")
	serverAttachIPv4Cmd.MarkFlagRequired("ipv4")

	serverAttachIPv6Cmd.Flags().StringVar(&serverAttachIPv6Slug, "ipv6", "", "IPv6 ID or slug (required)")
	serverAttachIPv6Cmd.MarkFlagRequired("ipv6")

	serverProtectCmd.Flags().BoolVar(&serverProtectEnable, "enable", true, "Enable or disable (--enable=false) destroy protection")

	serverSnapshotCreateCmd.Flags().StringVar(&snapshotCreateName, "name", "", "Snapshot name (required)")
	serverSnapshotCreateCmd.MarkFlagRequired("name")

	snapshotCmd.AddCommand(serverSnapshotListCmd)
	snapshotCmd.AddCommand(serverSnapshotCreateCmd)
	snapshotCmd.AddCommand(serverSnapshotDeleteCmd)
	snapshotCmd.AddCommand(serverSnapshotRollbackCmd)

	serverCmd.AddCommand(serverListCmd)
	serverCmd.AddCommand(serverGetCmd)
	serverCmd.AddCommand(serverCreateCmd)
	serverCmd.AddCommand(serverDeleteCmd)
	serverCmd.AddCommand(serverPowerCmd)
	serverCmd.AddCommand(serverConsoleCmd)
	serverCmd.AddCommand(serverAttachIPv4Cmd)
	serverCmd.AddCommand(serverAttachIPv6Cmd)
	serverCmd.AddCommand(serverProtectCmd)
	serverCmd.AddCommand(snapshotCmd)
}
