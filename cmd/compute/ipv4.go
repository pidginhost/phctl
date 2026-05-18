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

var ipv4Cmd = &cobra.Command{
	Use:     "ipv4",
	Aliases: []string{"ip"},
	Short:   "Manage IPv4 addresses",
	Args:    cobra.NoArgs,
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
			resp, _, err := c.CloudAPI.CloudIpv4List(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing IPv4 addresses", err)
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

var ipv4CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Allocate a new IPv4 address",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudIpv4Create(cmd.Context()).Execute()
		if err != nil {
			return cmdutil.APIError("creating IPv4", err)
		}
		cmd.Printf("IPv4 address created (ID: %d, Address: %s)\n", resp.Id, resp.Address)
		return nil
	},
}

var ipv4DeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete an IPv4 address",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete IPv4 address %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudIpv4Destroy(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("deleting IPv4", err)
		}
		cmd.Printf("IPv4 address %d deleted.\n", id)
		return nil
	},
}

var ipv4DetachCmd = &cobra.Command{
	Use:   "detach <id>",
	Short: "Detach an IPv4 address from its server",
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
		resp, _, err := c.CloudAPI.CloudIpv4DetachCreate(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("detaching IPv4", err)
		}
		cmd.Printf("IPv4 detached: %v\n", resp.Detached)
		return nil
	},
}

var ipv4ReverseDNSCmd = &cobra.Command{
	Use:     "reverse-dns <id>",
	Aliases: []string{"rdns"},
	Short:   "Get or set the PTR record for an IPv4 address",
	Long: "Without --hostname, prints the current PTR record. " +
		"With --hostname <fqdn>, sets the PTR record to that FQDN.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		hostname, err := cmd.Flags().GetString("hostname")
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("hostname") && hostname == "" {
			return fmt.Errorf("--hostname requires a non-empty FQDN")
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		var resp *pidginhost.ReverseDNS
		if cmd.Flags().Changed("hostname") {
			body := pidginhost.NewReverseDNS(hostname)
			resp, _, err = c.CloudAPI.CloudIpv4RdnsCreate(cmd.Context(), id).ReverseDNS(*body).Execute()
			if err != nil {
				return cmdutil.APIError("setting reverse DNS", err)
			}
		} else {
			resp, _, err = c.CloudAPI.CloudIpv4RdnsRetrieve(cmd.Context(), id).Execute()
			if err != nil {
				return cmdutil.APIError("fetching reverse DNS", err)
			}
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, resp, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "REVERSE_DNS")
			output.PrintRow(tw, id, resp.ReverseDns)
			tw.Flush()
		})
	},
}

func init() {
	ipv4ReverseDNSCmd.Flags().String("hostname", "", "Set PTR record to this FQDN (omit to read current value)")

	ipv4Cmd.AddCommand(ipv4ListCmd)
	ipv4Cmd.AddCommand(ipv4CreateCmd)
	ipv4Cmd.AddCommand(ipv4DeleteCmd)
	ipv4Cmd.AddCommand(ipv4DetachCmd)
	ipv4Cmd.AddCommand(ipv4ReverseDNSCmd)
}
