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

var ipv6Cmd = &cobra.Command{
	Use:   "ipv6",
	Short: "Manage IPv6 addresses",
	Args:  cobra.NoArgs,
}

var ipv6ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all IPv6 addresses",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		ips, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.PublicIPv6, bool, error) {
			resp, _, err := c.CloudAPI.CloudIpv6List(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing IPv6 addresses: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, ips, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "ADDRESS", "GATEWAY", "PREFIX", "ATTACHED", "SERVER")
			for _, ip := range ips {
				output.PrintRow(tw, ip.Id, ip.Address, ip.Gateway, ip.Prefix, ip.Attached, ip.Server)
			}
			tw.Flush()
		})
	},
}

var ipv6CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Allocate a new IPv6 address",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudIpv6Create(cmd.Context()).Execute()
		if err != nil {
			return fmt.Errorf("creating IPv6: %w", err)
		}
		cmd.Printf("IPv6 address created (ID: %d, Address: %s)\n", resp.Id, resp.Address)
		return nil
	},
}

var ipv6DeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete an IPv6 address",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete IPv6 address %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudIpv6Destroy(cmd.Context(), id).Execute()
		if err != nil {
			return fmt.Errorf("deleting IPv6: %w", err)
		}
		cmd.Printf("IPv6 address %d deleted.\n", id)
		return nil
	},
}

var ipv6DetachCmd = &cobra.Command{
	Use:   "detach <id>",
	Short: "Detach an IPv6 address from its server",
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
		resp, _, err := c.CloudAPI.CloudIpv6DetachCreate(cmd.Context(), id).Execute()
		if err != nil {
			return fmt.Errorf("detaching IPv6: %w", err)
		}
		cmd.Printf("IPv6 detached: %v\n", resp.Detached)
		return nil
	},
}

func init() {
	ipv6Cmd.AddCommand(ipv6ListCmd)
	ipv6Cmd.AddCommand(ipv6CreateCmd)
	ipv6Cmd.AddCommand(ipv6DeleteCmd)
	ipv6Cmd.AddCommand(ipv6DetachCmd)
}
