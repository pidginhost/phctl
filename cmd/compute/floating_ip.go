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

var floatingIPCmd = &cobra.Command{
	Use:     "floating-ip",
	Aliases: []string{"fip", "floating"},
	Short:   "Manage floating IPs (multi-VM HA)",
	Args:    cobra.NoArgs,
}

func floatingIPFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().Bool("ipv6", false, "Operate on floating IPv6 addresses (default: ipv4)")
}

func isIPv6(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("ipv6")
	if !v {
		v, _ = cmd.InheritedFlags().GetBool("ipv6")
	}
	return v
}

var floatingIPListCmd = &cobra.Command{
	Use:   "list",
	Short: "List floating IPs",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		format := cmdutil.OutputFormat(cmd)
		ipv6 := isIPv6(cmd)
		if ipv6 {
			ips, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.FloatingIPv6, bool, error) {
				resp, _, err := c.CloudAPI.CloudFloatingIpv6List(cmd.Context()).Page(page).Execute()
				if err != nil {
					return nil, false, err
				}
				return resp.Results, resp.Next.Get() != nil, nil
			})
			if err != nil {
				return cmdutil.APIError("listing floating IPv6", err)
			}
			return output.Print(cmd.OutOrStdout(), format, ips, func(w io.Writer) {
				tw := output.NewTabWriter(w)
				output.PrintRow(tw, "ID", "ADDRESS", "LABEL", "REVERSE_DNS", "AUTHORIZED")
				for _, ip := range ips {
					output.PrintRow(tw, ip.Id, ip.Address, ip.GetLabel(), ip.ReverseDns, ip.AuthorizedVmCount)
				}
				tw.Flush()
			})
		}
		ips, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.FloatingIPv4, bool, error) {
			resp, _, err := c.CloudAPI.CloudFloatingIpv4List(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing floating IPv4", err)
		}
		return output.Print(cmd.OutOrStdout(), format, ips, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "ADDRESS", "LABEL", "REVERSE_DNS", "AUTHORIZED")
			for _, ip := range ips {
				output.PrintRow(tw, ip.Id, ip.Address, ip.GetLabel(), ip.ReverseDns, ip.AuthorizedVmCount)
			}
			tw.Flush()
		})
	},
}

var floatingIPCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Allocate a new floating IP",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		label, _ := cmd.Flags().GetString("label")
		ipv6 := isIPv6(cmd)
		if ipv6 {
			body := pidginhost.FloatingIPv6Create{Label: &label}
			resp, _, err := c.CloudAPI.CloudFloatingIpv6Create(cmd.Context()).FloatingIPv6Create(body).Execute()
			if err != nil {
				return cmdutil.APIError("creating floating IPv6", err)
			}
			cmd.Printf("Floating IPv6 created (ID: %d, Address: %s)\n", resp.Id, resp.Address)
			return nil
		}
		body := pidginhost.FloatingIPv4Create{Label: &label}
		resp, _, err := c.CloudAPI.CloudFloatingIpv4Create(cmd.Context()).FloatingIPv4Create(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating floating IPv4", err)
		}
		cmd.Printf("Floating IPv4 created (ID: %d, Address: %s)\n", resp.Id, resp.Address)
		return nil
	},
}

var floatingIPDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a floating IP and revoke all authorizations",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete floating IP %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		if isIPv6(cmd) {
			_, err = c.CloudAPI.CloudFloatingIpv6Destroy(cmd.Context(), id).Execute()
		} else {
			_, err = c.CloudAPI.CloudFloatingIpv4Destroy(cmd.Context(), id).Execute()
		}
		if err != nil {
			return cmdutil.APIError("deleting floating IP", err)
		}
		cmd.Printf("Floating IP %d deleted.\n", id)
		return nil
	},
}

var floatingIPAuthorizeCmd = &cobra.Command{
	Use:   "authorize <id> <server-id>",
	Short: "Authorize a server to use a floating IP",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		serverID, err := cmdutil.ParseInt32(args[1])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := pidginhost.FloatingIPAuthorizeRequest{ServerId: serverID}
		if isIPv6(cmd) {
			_, _, err = c.CloudAPI.CloudFloatingIpv6AuthorizeCreate(cmd.Context(), id).FloatingIPAuthorizeRequest(body).Execute()
		} else {
			_, _, err = c.CloudAPI.CloudFloatingIpv4AuthorizeCreate(cmd.Context(), id).FloatingIPAuthorizeRequest(body).Execute()
		}
		if err != nil {
			return cmdutil.APIError(fmt.Sprintf("authorizing server %d for floating IP %d", serverID, id), err)
		}
		cmd.Printf("Server %d authorized for floating IP %d.\n", serverID, id)
		return nil
	},
}

var floatingIPUnauthorizeCmd = &cobra.Command{
	Use:     "unauthorize <id> <server-id>",
	Aliases: []string{"revoke"},
	Short:   "Revoke a server's authorization for a floating IP",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		serverID, err := cmdutil.ParseInt32(args[1])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := pidginhost.FloatingIPAuthorizeRequest{ServerId: serverID}
		if isIPv6(cmd) {
			_, _, err = c.CloudAPI.CloudFloatingIpv6UnauthorizeCreate(cmd.Context(), id).FloatingIPAuthorizeRequest(body).Execute()
		} else {
			_, _, err = c.CloudAPI.CloudFloatingIpv4UnauthorizeCreate(cmd.Context(), id).FloatingIPAuthorizeRequest(body).Execute()
		}
		if err != nil {
			return cmdutil.APIError(fmt.Sprintf("unauthorizing server %d for floating IP %d", serverID, id), err)
		}
		cmd.Printf("Server %d unauthorized for floating IP %d.\n", serverID, id)
		return nil
	},
}

var floatingIPAuthorizationsCmd = &cobra.Command{
	Use:   "authorizations <id>",
	Short: "List servers authorized for a floating IP",
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
		var auths []pidginhost.FloatingIPAuthorization
		if isIPv6(cmd) {
			auths, err = cmdutil.FetchAll(func(page int32) ([]pidginhost.FloatingIPAuthorization, bool, error) {
				resp, _, err := c.CloudAPI.CloudFloatingIpv6AuthorizationsList(cmd.Context(), id).Page(page).Execute()
				if err != nil {
					return nil, false, err
				}
				return resp.Results, resp.Next.Get() != nil, nil
			})
		} else {
			auths, err = cmdutil.FetchAll(func(page int32) ([]pidginhost.FloatingIPAuthorization, bool, error) {
				resp, _, err := c.CloudAPI.CloudFloatingIpv4AuthorizationsList(cmd.Context(), id).Page(page).Execute()
				if err != nil {
					return nil, false, err
				}
				return resp.Results, resp.Next.Get() != nil, nil
			})
		}
		if err != nil {
			return cmdutil.APIError("listing authorizations", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, auths, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "SERVER_ID", "SERVER", "AUTHORIZED_AT")
			for _, a := range auths {
				output.PrintRow(tw, a.Id, a.ServerId, a.ServerHostname, a.CreatedAt)
			}
			tw.Flush()
		})
	},
}

var floatingIPReverseDNSCmd = &cobra.Command{
	Use:     "reverse-dns <id>",
	Aliases: []string{"rdns"},
	Short:   "Get or set the PTR record for a floating IP",
	Long: "Without --hostname, prints the current PTR record. " +
		"With --hostname <fqdn>, sets it. Use --ipv6 to target a floating IPv6.",
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
		ipv6 := isIPv6(cmd)
		if cmd.Flags().Changed("hostname") {
			body := pidginhost.NewReverseDNS(hostname)
			if ipv6 {
				resp, _, err = c.CloudAPI.CloudFloatingIpv6RdnsCreate(cmd.Context(), id).ReverseDNS(*body).Execute()
			} else {
				resp, _, err = c.CloudAPI.CloudFloatingIpv4RdnsCreate(cmd.Context(), id).ReverseDNS(*body).Execute()
			}
			if err != nil {
				return cmdutil.APIError("setting floating IP reverse DNS", err)
			}
		} else {
			if ipv6 {
				resp, _, err = c.CloudAPI.CloudFloatingIpv6RdnsRetrieve(cmd.Context(), id).Execute()
			} else {
				resp, _, err = c.CloudAPI.CloudFloatingIpv4RdnsRetrieve(cmd.Context(), id).Execute()
			}
			if err != nil {
				return cmdutil.APIError("fetching floating IP reverse DNS", err)
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
	floatingIPFlags(floatingIPCmd)
	floatingIPCreateCmd.Flags().String("label", "", "Optional label (e.g. ha-mysql-vip)")
	floatingIPReverseDNSCmd.Flags().String("hostname", "", "Set PTR record to this FQDN (omit to read current value)")
	floatingIPCmd.AddCommand(floatingIPListCmd)
	floatingIPCmd.AddCommand(floatingIPCreateCmd)
	floatingIPCmd.AddCommand(floatingIPDeleteCmd)
	floatingIPCmd.AddCommand(floatingIPAuthorizeCmd)
	floatingIPCmd.AddCommand(floatingIPUnauthorizeCmd)
	floatingIPCmd.AddCommand(floatingIPAuthorizationsCmd)
	floatingIPCmd.AddCommand(floatingIPReverseDNSCmd)
}
