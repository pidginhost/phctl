package compute

import (
	"fmt"
	"io"
	"os"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

const userDataMaxBytes = 65536

var serverCmd = &cobra.Command{
	Use:     "server",
	Aliases: []string{"s"},
	Short:   "Manage cloud servers",
	Args:    cobra.NoArgs,
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
			resp, _, err := c.CloudAPI.CloudServersList(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing servers", err)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), servers, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME", "IMAGE", "PACKAGE", "STATUS")
			for _, s := range servers {
				output.PrintRow(tw, s.Id, output.Pstr(s.Hostname), s.Image, s.Package, output.Pstr(s.Status))
			}
			tw.Flush()
		})
	},
}

func printServerDetailsTable(w io.Writer, s *pidginhost.ServerDetail) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "ID:", s.Id)
	output.PrintRow(tw, "Hostname:", s.Hostname)
	output.PrintRow(tw, "Image:", s.Image)
	output.PrintRow(tw, "Package:", s.Package)
	output.PrintRow(tw, "CPUs:", s.Cpus)
	output.PrintRow(tw, "Memory:", s.Memory)
	output.PrintRow(tw, "Disk:", s.DiskSize)
	output.PrintRow(tw, "Status:", s.Status)
	output.PrintRow(tw, "Username:", s.Username)
	output.PrintRow(tw, "Destroy Protection:", s.DestroyProtection)
	output.PrintRow(tw, "HA Enabled:", s.HaEnabled)
	tw.Flush()
	if len(s.FloatingIps) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Floating IPs:")
		ftw := output.NewTabWriter(w)
		output.PrintRow(ftw, "ID", "VERSION", "ADDRESS", "LABEL", "REVERSE_DNS")
		for _, f := range s.FloatingIps {
			output.PrintRow(ftw, f.Id, f.Version, f.Address, f.Label, f.ReverseDns)
		}
		ftw.Flush()
	}
}

var serverGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get server details",
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
		s, _, err := c.CloudAPI.CloudServersRetrieve(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("getting server", err)
		}

		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), s, func(w io.Writer) {
			printServerDetailsTable(w, s)
		})
	},
}

// resolveUserData picks the cloud-init payload from --user-data or
// --user-data-file. Cobra enforces mutual exclusion. A path of "-"
// reads from stdin (the caller passes cmd.InOrStdin() so tests can
// inject a reader). The size cap mirrors the API's 64 KiB limit so we
// fail before the round trip.
func resolveUserData(inline, path string, stdin io.Reader) (string, error) {
	if inline != "" {
		if len(inline) > userDataMaxBytes {
			return "", fmt.Errorf("--user-data exceeds %d bytes", userDataMaxBytes)
		}
		return inline, nil
	}
	if path == "" {
		return "", nil
	}
	var (
		data []byte
		err  error
	)
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", fmt.Errorf("reading user-data: %w", err)
	}
	if len(data) > userDataMaxBytes {
		return "", fmt.Errorf("user-data exceeds %d bytes", userDataMaxBytes)
	}
	return string(data), nil
}

var (
	serverCreateImage          string
	serverCreatePackage        string
	serverCreateGeneration     string
	serverCreateHostname       string
	serverCreateProject        string
	serverCreateSSHKeyID       string
	serverCreatePassword       string
	serverCreateNewIPv4        bool
	serverCreateNoPubIPv4Ack   bool
	serverCreatePrivateNetwork string
	serverCreatePrivateAddress string
	serverCreateUserData       string
	serverCreateUserDataFile   string
)

var serverCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new server",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}

		userData, err := resolveUserData(serverCreateUserData, serverCreateUserDataFile, cmd.InOrStdin())
		if err != nil {
			return err
		}

		body := *pidginhost.NewServerAdd(serverCreateImage, serverCreatePackage)
		if serverCreateGeneration != "" {
			body.Generation = pidginhost.PtrString(serverCreateGeneration)
		}
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
		if serverCreateNoPubIPv4Ack {
			body.NoNetworkAcknowledged = pidginhost.PtrBool(true)
		}
		if serverCreatePrivateNetwork != "" {
			body.PrivateNetwork = pidginhost.PtrString(serverCreatePrivateNetwork)
		}
		if serverCreatePrivateAddress != "" {
			body.PrivateAddress = pidginhost.PtrString(serverCreatePrivateAddress)
		}
		if userData != "" {
			body.UserData = pidginhost.PtrString(userData)
		}

		resp, _, err := c.CloudAPI.CloudServersCreate(cmd.Context()).ServerAdd(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating server", err)
		}

		cmd.Printf("Server created (ID: %d)\n", resp.Id)
		return nil
	},
}

var serverDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a server",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete server %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudServersDestroy(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("deleting server", err)
		}
		cmd.Printf("Server %d deleted.\n", id)
		return nil
	},
}

var serverPowerAction string

var serverPowerCmd = &cobra.Command{
	Use:   "power <id>",
	Short: "Manage server power (--action start|stop|shutdown|reboot)",
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
		body := *pidginhost.NewPowerManagementRequest(pidginhost.PowerManagementRequestActionEnum(serverPowerAction))
		_, _, err = c.CloudAPI.CloudServersPowerManagementCreate(cmd.Context(), id).PowerManagementRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("power management", err)
		}
		cmd.Printf("Power action '%s' executed on server %d.\n", serverPowerAction, id)
		return nil
	},
}

var serverConsoleCmd = &cobra.Command{
	Use:   "console <id>",
	Short: "Get server console token",
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
		resp, _, err := c.CloudAPI.CloudServersConsoleCreate(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("getting console", err)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp, func(w io.Writer) {
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
	Long: "Attach an IPv4 address to a server. The first IPv4 lands on the primary NIC; " +
		"subsequent attaches add a new secondary NIC carrying just that IPv4.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewAttachIPv4(serverAttachIPv4Slug)
		resp, _, err := c.CloudAPI.CloudServersAttachIpv4Create(cmd.Context(), id).AttachIPv4(body).Execute()
		if err != nil {
			return cmdutil.APIError("attaching IPv4", err)
		}
		if resp != nil && !resp.Attached {
			return fmt.Errorf("attaching IPv4: backend reported the IPv4 was not attached")
		}
		cmd.Printf("IPv4 %s attached to server %d.\n", serverAttachIPv4Slug, id)
		return nil
	},
}

var serverDetachIPv4Slug string

var serverDetachIPv4Cmd = &cobra.Command{
	Use:   "detach-ipv4 <server-id>",
	Short: "Detach an IPv4 address from a server",
	Long: "Detach an IPv4 address from a server. Without --ipv4, the primary NIC's IPv4 " +
		"is detached. Pass --ipv4 <id|slug> to target a specific attached address " +
		"(required when the server has more than one IPv4).",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		req := c.CloudAPI.CloudServersDetachIpv4Create(cmd.Context(), id)
		if serverDetachIPv4Slug != "" {
			req = req.Ipv4(serverDetachIPv4Slug)
		}
		resp, _, err := req.Execute()
		if err != nil {
			return cmdutil.APIError("detaching IPv4", err)
		}
		if resp != nil && !resp.Detached {
			return fmt.Errorf("detaching IPv4: backend reported the IPv4 was not detached")
		}
		if serverDetachIPv4Slug != "" {
			cmd.Printf("IPv4 %s detached from server %d.\n", serverDetachIPv4Slug, id)
		} else {
			cmd.Printf("Primary IPv4 detached from server %d.\n", id)
		}
		return nil
	},
}

var serverAttachIPv6Slug string

var serverAttachIPv6Cmd = &cobra.Command{
	Use:   "attach-ipv6 <server-id>",
	Short: "Attach an IPv6 address to a server",
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
		body := *pidginhost.NewAttachIPv6(serverAttachIPv6Slug)
		_, _, err = c.CloudAPI.CloudServersAttachIpv6Create(cmd.Context(), id).AttachIPv6(body).Execute()
		if err != nil {
			return cmdutil.APIError("attaching IPv6", err)
		}
		cmd.Printf("IPv6 %s attached to server %d.\n", serverAttachIPv6Slug, id)
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
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewDestroyProtection(serverProtectEnable)
		_, _, err = c.CloudAPI.CloudServersDestroyProtectionCreate(cmd.Context(), id).DestroyProtection(body).Execute()
		if err != nil {
			return cmdutil.APIError("setting destroy protection", err)
		}
		state := "enabled"
		if !serverProtectEnable {
			state = "disabled"
		}
		cmd.Printf("Destroy protection %s on server %d.\n", state, id)
		return nil
	},
}

// --- Snapshots ---

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage server snapshots",
	Args:  cobra.NoArgs,
}

var serverSnapshotListCmd = &cobra.Command{
	Use:   "list <server-id>",
	Short: "List snapshots for a server",
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
		snapshots, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.Snapshot, bool, error) {
			resp, _, err := c.CloudAPI.CloudServersSnapshotsList(cmd.Context(), id).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing snapshots", err)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), snapshots, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "NAME", "STATE", "CREATED")
			for _, s := range snapshots {
				created := "<none>"
				if t := s.CreatedAt.Get(); t != nil {
					created = *t
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
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewSnapshotCreate(snapshotCreateName)
		_, _, err = c.CloudAPI.CloudServersSnapshotsCreate(cmd.Context(), id).SnapshotCreate(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating snapshot", err)
		}
		cmd.Printf("Snapshot '%s' creation queued.\n", snapshotCreateName)
		return nil
	},
}

var serverSnapshotDeleteCmd = &cobra.Command{
	Use:     "delete <server-id> <snapshot-name>",
	Aliases: []string{"rm"},
	Short:   "Delete a snapshot",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete snapshot '%s' from server %d?", args[1], id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, _, err = c.CloudAPI.CloudServersSnapshotsDestroy(cmd.Context(), id, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("deleting snapshot", err)
		}
		cmd.Printf("Snapshot '%s' deletion queued.\n", args[1])
		return nil
	},
}

var serverSnapshotRollbackCmd = &cobra.Command{
	Use:   "rollback <server-id> <snapshot-name>",
	Short: "Rollback a server to a snapshot",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Rollback server %d to snapshot '%s'?", id, args[1])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, _, err = c.CloudAPI.CloudServersSnapshotsRollbackCreate(cmd.Context(), id, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("rolling back snapshot", err)
		}
		cmd.Printf("Rollback to snapshot '%s' queued.\n", args[1])
		return nil
	},
}

func init() {
	serverCreateCmd.Flags().StringVar(&serverCreateImage, "image", "", "OS image (required)")
	serverCreateCmd.Flags().StringVar(&serverCreatePackage, "package", "", "Server package (required)")
	serverCreateCmd.Flags().StringVar(&serverCreateGeneration, "generation", "", "Hardware generation slug (e.g. compute-optimized). Defaults to the backend's default generation.")
	serverCreateCmd.Flags().StringVar(&serverCreateHostname, "hostname", "", "Server hostname")
	serverCreateCmd.Flags().StringVar(&serverCreateProject, "project", "", "Project name")
	serverCreateCmd.Flags().StringVar(&serverCreateSSHKeyID, "ssh-key-id", "", "SSH key ID to inject")
	serverCreateCmd.Flags().StringVar(&serverCreatePassword, "password", "", "Root password")
	serverCreateCmd.Flags().BoolVar(&serverCreateNewIPv4, "new-ipv4", false, "Allocate a new public IPv4")
	serverCreateCmd.Flags().BoolVar(&serverCreateNoPubIPv4Ack, "no-public-ipv4-ack", false, "Acknowledge creating the server without a public IPv4 or IPv6. Required when no public network is requested on packages where the backend would otherwise reject the create.")
	serverCreateCmd.Flags().StringVar(&serverCreatePrivateNetwork, "private-network", "", "Attach to this private network at create time (ID or CIDR slug). Pair with --private-address for a specific IP.")
	serverCreateCmd.Flags().StringVar(&serverCreatePrivateAddress, "private-address", "", "Static IPv4 inside --private-network. Leave empty for auto-assign.")
	serverCreateCmd.Flags().StringVar(&serverCreateUserData, "user-data", "", "Cloud-init startup script body (Linux only, mutually exclusive with --user-data-file)")
	serverCreateCmd.Flags().StringVar(&serverCreateUserDataFile, "user-data-file", "", "Path to cloud-init startup script (use '-' for stdin)")
	serverCreateCmd.MarkFlagsMutuallyExclusive("user-data", "user-data-file")
	serverCreateCmd.MarkFlagRequired("image")
	serverCreateCmd.MarkFlagRequired("package")

	serverPowerCmd.Flags().StringVar(&serverPowerAction, "action", "", "Power action: start, stop, shutdown, reboot")
	serverPowerCmd.MarkFlagRequired("action")

	serverAttachIPv4Cmd.Flags().StringVar(&serverAttachIPv4Slug, "ipv4", "", "IPv4 ID or slug (required)")
	serverAttachIPv4Cmd.MarkFlagRequired("ipv4")

	serverDetachIPv4Cmd.Flags().StringVar(&serverDetachIPv4Slug, "ipv4", "", "IPv4 ID or slug to detach. Required when the server has more than one IPv4; omit to detach the primary NIC's IPv4 on single-IPv4 servers.")

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
	serverCmd.AddCommand(serverDetachIPv4Cmd)
	serverCmd.AddCommand(serverAttachIPv6Cmd)
	serverCmd.AddCommand(serverProtectCmd)
	serverCmd.AddCommand(snapshotCmd)
}
