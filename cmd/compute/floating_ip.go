package compute

import (
	"fmt"
	"io"
	"net/http"

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

type floatingIP struct {
	Id                int32  `json:"id" yaml:"id"`
	Address           string `json:"address" yaml:"address"`
	Gateway           string `json:"gateway" yaml:"gateway"`
	Prefix            int32  `json:"prefix" yaml:"prefix"`
	Label             string `json:"label" yaml:"label"`
	AuthorizedVmCount int32  `json:"authorized_vm_count" yaml:"authorized_vm_count"`
	CreatedAt         string `json:"created_at" yaml:"created_at"`
}

type floatingIPCreateRequest struct {
	Label string `json:"label,omitempty"`
}

type floatingIPAuthorizeRequest struct {
	ServerId int32 `json:"server_id"`
}

type floatingIPAuthorization struct {
	Id             int32  `json:"id" yaml:"id"`
	ServerId       int32  `json:"server_id" yaml:"server_id"`
	ServerHostname string `json:"server_hostname" yaml:"server_hostname"`
	CreatedAt      string `json:"created_at" yaml:"created_at"`
}

func floatingIPFamily(ipv6 bool) string {
	if ipv6 {
		return "IPv6"
	}
	return "IPv4"
}

func floatingIPCollectionPath(ipv6 bool) string {
	if ipv6 {
		return "/api/cloud/floating-ipv6/"
	}
	return "/api/cloud/floating-ipv4/"
}

func floatingIPDetailPath(ipv6 bool, id int32) string {
	return fmt.Sprintf("%s%d/", floatingIPCollectionPath(ipv6), id)
}

func floatingIPActionPath(ipv6 bool, id int32, action string) string {
	return fmt.Sprintf("%s%s/", floatingIPDetailPath(ipv6, id), action)
}

var floatingIPListCmd = &cobra.Command{
	Use:   "list",
	Short: "List floating IPs",
	RunE: func(cmd *cobra.Command, args []string) error {
		ipv6 := isIPv6(cmd)
		ips, err := client.RawFetchAll[floatingIP](cmd.Context(), floatingIPCollectionPath(ipv6))
		if err != nil {
			return fmt.Errorf("listing floating %s: %w", floatingIPFamily(ipv6), err)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), ips, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "ADDRESS", "LABEL", "AUTHORIZED")
			for _, ip := range ips {
				output.PrintRow(tw, ip.Id, ip.Address, ip.Label, ip.AuthorizedVmCount)
			}
			tw.Flush()
		})
	},
}

var floatingIPCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Allocate a new floating IP",
	RunE: func(cmd *cobra.Command, args []string) error {
		ipv6 := isIPv6(cmd)
		label, _ := cmd.Flags().GetString("label")
		req := floatingIPCreateRequest{Label: label}
		var resp floatingIP
		if err := client.RawPost(cmd.Context(), floatingIPCollectionPath(ipv6), req, &resp, http.StatusCreated); err != nil {
			return fmt.Errorf("creating floating %s: %w", floatingIPFamily(ipv6), err)
		}
		cmd.Printf("Floating %s created (ID: %d, Address: %s)\n", floatingIPFamily(ipv6), resp.Id, resp.Address)
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
		ipv6 := isIPv6(cmd)
		if err := client.RawDelete(cmd.Context(), floatingIPDetailPath(ipv6, id)); err != nil {
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
		ipv6 := isIPv6(cmd)
		body := floatingIPAuthorizeRequest{ServerId: serverID}
		if err := client.RawPost(cmd.Context(), floatingIPActionPath(ipv6, id, "authorize"), body, nil); err != nil {
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
		ipv6 := isIPv6(cmd)
		body := floatingIPAuthorizeRequest{ServerId: serverID}
		if err := client.RawPost(cmd.Context(), floatingIPActionPath(ipv6, id, "unauthorize"), body, nil); err != nil {
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
		ipv6 := isIPv6(cmd)
		var auths []floatingIPAuthorization
		if err := client.RawGet(cmd.Context(), floatingIPActionPath(ipv6, id, "authorizations"), &auths); err != nil {
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
