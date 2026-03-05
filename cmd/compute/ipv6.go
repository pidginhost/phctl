package compute

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var ipv6Cmd = &cobra.Command{
	Use:   "ipv6",
	Short: "Manage IPv6 addresses",
}

var ipv6ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all IPv6 addresses",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudIpv6List(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing IPv6 addresses: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, resp.Results, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "ADDRESS", "GATEWAY", "PREFIX", "ATTACHED", "SERVER")
			for _, ip := range resp.Results {
				output.PrintRow(tw, ip.Id, ip.Address, ip.Gateway, ip.Prefix, ip.Attached, ip.Server)
			}
			tw.Flush()
		})
		return nil
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
		resp, _, err := c.CloudAPI.CloudIpv6Create(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("creating IPv6: %w", err)
		}
		fmt.Printf("IPv6 address created (ID: %d, Address: %s)\n", resp.Id, resp.Address)
		return nil
	},
}

var ipv6DeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete an IPv6 address",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete IPv6 address %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudIpv6Destroy(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("deleting IPv6: %w", err)
		}
		fmt.Printf("IPv6 address %d deleted.\n", id)
		return nil
	},
}

var ipv6DetachCmd = &cobra.Command{
	Use:   "detach <id>",
	Short: "Detach an IPv6 address from its server",
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
		resp, _, err := c.CloudAPI.CloudIpv6DetachCreate(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("detaching IPv6: %w", err)
		}
		fmt.Printf("IPv6 detached: %v\n", resp.Detached)
		return nil
	},
}

func init() {
	ipv6Cmd.AddCommand(ipv6ListCmd)
	ipv6Cmd.AddCommand(ipv6CreateCmd)
	ipv6Cmd.AddCommand(ipv6DeleteCmd)
	ipv6Cmd.AddCommand(ipv6DetachCmd)
}
