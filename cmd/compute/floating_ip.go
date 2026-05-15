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
		// also inherit parent
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
		if isIPv6(cmd) {
			ips, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.FloatingIPv6, bool, error) {
				resp, _, err := c.CloudAPI.CloudFloatingIpv6List(cmd.Context()).Page(page).Execute()
				if err != nil {
					return nil, false, err
				}
				return resp.Results, resp.Next.Get() != nil, nil
			})
			if err != nil {
				return fmt.Errorf("listing floating IPv6: %w", err)
			}
			return output.Print(cmd.OutOrStdout(), format, ips, func(w io.Writer) {
				tw := output.NewTabWriter(w)
				output.PrintRow(tw, "ID", "ADDRESS", "LABEL", "AUTHORIZED")
				for _, ip := range ips {
					output.PrintRow(tw, ip.Id, ip.Address, ip.GetLabel(), ip.AuthorizedVmCount)
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
			return fmt.Errorf("listing floating IPv4: %w", err)
		}
		return output.Print(cmd.OutOrStdout(), format, ips, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "ADDRESS", "LABEL", "AUTHORIZED")
			for _, ip := range ips {
				output.PrintRow(tw, ip.Id, ip.Address, ip.GetLabel(), ip.AuthorizedVmCount)
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
		if isIPv6(cmd) {
			req := pidginhost.FloatingIPv6Create{Label: &label}
			resp, _, err := c.CloudAPI.CloudFloatingIpv6Create(cmd.Context()).FloatingIPv6Create(req).Execute()
			if err != nil {
				return fmt.Errorf("creating floating IPv6: %w", err)
			}
			cmd.Printf("Floating IPv6 created (ID: %d, Address: %s)\n", resp.Id, resp.Address)
			return nil
		}
		req := pidginhost.FloatingIPv4Create{Label: &label}
		resp, _, err := c.CloudAPI.CloudFloatingIpv4Create(cmd.Context()).FloatingIPv4Create(req).Execute()
		if err != nil {
			return fmt.Errorf("creating floating IPv4: %w", err)
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
			return fmt.Errorf("deleting floating IP: %w", err)
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
			return fmt.Errorf("authorizing server %d for floating IP %d: %w", serverID, id, err)
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
			return fmt.Errorf("unauthorizing server %d for floating IP %d: %w", serverID, id, err)
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
			auths, _, err = c.CloudAPI.CloudFloatingIpv6AuthorizationsList(cmd.Context(), id).Execute()
		} else {
			auths, _, err = c.CloudAPI.CloudFloatingIpv4AuthorizationsList(cmd.Context(), id).Execute()
		}
		if err != nil {
			return fmt.Errorf("listing authorizations: %w", err)
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

func init() {
	floatingIPFlags(floatingIPCmd)
	floatingIPCreateCmd.Flags().String("label", "", "Optional label (e.g. ha-mysql-vip)")
	floatingIPCmd.AddCommand(floatingIPListCmd)
	floatingIPCmd.AddCommand(floatingIPCreateCmd)
	floatingIPCmd.AddCommand(floatingIPDeleteCmd)
	floatingIPCmd.AddCommand(floatingIPAuthorizeCmd)
	floatingIPCmd.AddCommand(floatingIPUnauthorizeCmd)
	floatingIPCmd.AddCommand(floatingIPAuthorizationsCmd)
}
