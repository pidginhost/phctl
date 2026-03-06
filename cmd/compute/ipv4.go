package compute

import (
	"context"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var ipv4Cmd = &cobra.Command{
	Use:     "ipv4",
	Aliases: []string{"ip"},
	Short:   "Manage IPv4 addresses",
}

var ipv4ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all IPv4 addresses",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		ips, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.PublicIPv4, bool, error) {
			resp, _, err := c.CloudAPI.CloudIpv4List(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing IPv4 addresses: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, ips, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "ADDRESS", "GATEWAY", "PREFIX", "ATTACHED", "SERVER")
			for _, ip := range ips {
				output.PrintRow(tw, ip.Id, ip.Address, ip.Gateway, ip.Prefix, ip.Attached, ip.Server)
			}
			tw.Flush()
		})
	},
}

var ipv4CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Allocate a new IPv4 address",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudIpv4Create(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("creating IPv4: %w", err)
		}
		fmt.Printf("IPv4 address created (ID: %d, Address: %s)\n", resp.Id, resp.Address)
		return nil
	},
}

var ipv4DeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete an IPv4 address",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete IPv4 address %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudIpv4Destroy(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("deleting IPv4: %w", err)
		}
		fmt.Printf("IPv4 address %d deleted.\n", id)
		return nil
	},
}

var ipv4DetachCmd = &cobra.Command{
	Use:   "detach <id>",
	Short: "Detach an IPv4 address from its server",
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
		resp, _, err := c.CloudAPI.CloudIpv4DetachCreate(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("detaching IPv4: %w", err)
		}
		fmt.Printf("IPv4 detached: %v\n", resp.Detached)
		return nil
	},
}

func init() {
	ipv4Cmd.AddCommand(ipv4ListCmd)
	ipv4Cmd.AddCommand(ipv4CreateCmd)
	ipv4Cmd.AddCommand(ipv4DeleteCmd)
	ipv4Cmd.AddCommand(ipv4DetachCmd)
}
