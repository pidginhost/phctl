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

var networkCmd = &cobra.Command{
	Use:     "network",
	Aliases: []string{"net"},
	Short:   "Manage private networks",
	Args:    cobra.NoArgs,
}

var networkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all private networks",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		networks, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.PrivateNetwork, bool, error) {
			resp, _, err := c.CloudAPI.CloudPrivateNetworksList(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			if resp == nil {
				return nil, false, fmt.Errorf("server returned no response page")
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing networks", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, networks, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "SLUG", "ADDRESS", "PROVISIONED", "SERVERS")
			for _, n := range networks {
				output.PrintRow(tw, n.Id, n.Slug, n.Address, n.Provisioned, len(n.Servers))
			}
			tw.Flush()
		})
	},
}

var networkGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get private network details",
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
		net, _, err := c.CloudAPI.CloudPrivateNetworksRetrieve(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("getting network", err)
		}
		if net == nil {
			return fmt.Errorf("getting network: server returned no response")
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, net, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", net.Id)
			output.PrintRow(tw, "Slug:", net.Slug)
			output.PrintRow(tw, "Address:", net.Address)
			output.PrintRow(tw, "Provisioned:", net.Provisioned)
			tw.Flush()
			if len(net.Servers) > 0 {
				fmt.Fprintln(w)
				fmt.Fprintln(w, "Servers:")
				for _, s := range net.Servers {
					for k, v := range s {
						fmt.Fprintf(w, "  %s: %s\n", k, v)
					}
				}
			}
		})
	},
}

var networkCreateAddress string

var networkCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a private network",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewPrivateNetworkRequest(networkCreateAddress)
		resp, _, err := c.CloudAPI.CloudPrivateNetworksCreate(cmd.Context()).PrivateNetworkRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating network", err)
		}
		if resp == nil {
			return fmt.Errorf("creating network: server returned no response")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"Private network created (ID: %d, Address: %s)\n", resp.Id, resp.Address)
	},
}

var networkDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a private network",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete private network %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudPrivateNetworksDestroy(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("deleting network", err)
		}
		cmd.Printf("Private network %d deleted.\n", id)
		return nil
	},
}

var (
	networkAddServerHost    string
	networkAddServerAddress string
)

var networkAddServerCmd = &cobra.Command{
	Use:   "add-server <network-id>",
	Short: "Add a server to a private network",
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
		body := *pidginhost.NewPrivateNetworkAddHostRequest(networkAddServerHost)
		if networkAddServerAddress != "" {
			body.Address = pidginhost.PtrString(networkAddServerAddress)
		}
		resp, _, err := c.CloudAPI.CloudPrivateNetworksAddServerCreate(cmd.Context(), id).PrivateNetworkAddHostRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("adding server to network", err)
		}
		if resp == nil {
			return fmt.Errorf("adding server to network: server returned no response")
		}
		if !resp.Created {
			return fmt.Errorf("adding server to network: server was not added to the network")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"Server added to network: %v\n", resp.Created)
	},
}

var networkRemoveServerHost string

var networkRemoveServerCmd = &cobra.Command{
	Use:   "remove-server <network-id>",
	Short: "Remove a server from a private network",
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
		body := *pidginhost.NewPrivateNetworkRemoveHostRequest(networkRemoveServerHost)
		resp, _, err := c.CloudAPI.CloudPrivateNetworksRemoveServerCreate(cmd.Context(), id).PrivateNetworkRemoveHostRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("removing server from network", err)
		}
		if resp == nil {
			return fmt.Errorf("removing server from network: server returned no response")
		}
		if !resp.Removed {
			return fmt.Errorf("removing server from network: server was not removed from the network")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"Server removed from network: %v\n", resp.Removed)
	},
}

var networkRouteStatusCmd = &cobra.Command{
	Use:   "route-status <network-id>",
	Short: "Show each server's default route on a network",
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
		status, _, err := c.CloudAPI.CloudPrivateNetworksRouteStatusRetrieve(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("getting route status", err)
		}
		if status == nil {
			return fmt.Errorf("getting route status: server returned no response")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), status, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "Network:", status.Network)
			output.PrintRow(tw, "Gateway:", output.Pstr(status.Gateway.Get()))
			tw.Flush()
			if len(status.Servers) > 0 {
				fmt.Fprintln(w)
				tw = output.NewTabWriter(w)
				output.PrintRow(tw, "SERVER", "HOSTNAME", "POWER", "CONFIG", "AFTER REBOOT", "ROUTES HERE", "ACKNOWLEDGED")
				for _, s := range status.Servers {
					output.PrintRow(tw, s.Server, s.Hostname, s.Power, s.Config, output.Pstr(s.Pending.Get()), s.RoutesHere, s.Acknowledged)
				}
				tw.Flush()
			}
			if len(status.Operations) > 0 {
				fmt.Fprintln(w)
				tw = output.NewTabWriter(w)
				output.PrintRow(tw, "OPERATION", "STATUS", "STARTED", "COMPLETED", "REASON")
				for _, op := range status.Operations {
					output.PrintRow(tw, op.Id, op.Status, op.StartedAt, output.Pstr(op.CompletedAt.Get()), op.Reason)
				}
				tw.Flush()
			}
		})
	},
}

var networkApplyRouteCmd = &cobra.Command{
	Use:   "apply-route <network-id>",
	Short: "Apply the network's route to servers and reboot them",
	Long: `Apply the network's route to the named servers and reboot them.

'network route-status' shows which servers need it and, afterwards, how the
apply went.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		servers, _ := cmd.Flags().GetInt32Slice("servers")
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Apply the route of network %d to servers %v? They reboot.", id, servers)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewApplyRouteRequest(servers)
		op, _, err := c.CloudAPI.CloudPrivateNetworksApplyRouteCreate(cmd.Context(), id).ApplyRouteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("applying route", err)
		}
		return queued(cmd, fmt.Sprintf("Applying the route of network %d", id), op)
	},
}

func init() {
	networkApplyRouteCmd.Flags().Int32Slice("servers", nil, "IDs of the servers to apply the route to and reboot (required)")
	networkApplyRouteCmd.MarkFlagRequired("servers")
	networkCmd.AddCommand(networkRouteStatusCmd)
	networkCmd.AddCommand(networkApplyRouteCmd)

	// The server assigns the slug, so --slug was never applied. Kept, deprecated,
	// so scripts that pass it do not break.
	networkCreateCmd.Flags().String("slug", "", "Ignored: the server assigns the slug")
	_ = networkCreateCmd.Flags().MarkDeprecated("slug", "the server assigns the slug; the value was never applied")
	networkCreateCmd.Flags().StringVar(&networkCreateAddress, "address", "", "Network address in CIDR format (required)")
	networkCreateCmd.MarkFlagRequired("address")

	networkAddServerCmd.Flags().StringVar(&networkAddServerHost, "server", "", "Server hostname (required)")
	networkAddServerCmd.Flags().StringVar(&networkAddServerAddress, "address", "", "Private IP address to assign")
	networkAddServerCmd.MarkFlagRequired("server")

	networkRemoveServerCmd.Flags().StringVar(&networkRemoveServerHost, "server", "", "Server hostname or private IP (required)")
	networkRemoveServerCmd.MarkFlagRequired("server")

	networkCmd.AddCommand(networkListCmd)
	networkCmd.AddCommand(networkGetCmd)
	networkCmd.AddCommand(networkCreateCmd)
	networkCmd.AddCommand(networkDeleteCmd)
	networkCmd.AddCommand(networkAddServerCmd)
	networkCmd.AddCommand(networkRemoveServerCmd)
}
