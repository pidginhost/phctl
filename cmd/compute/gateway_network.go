package compute

import (
	"fmt"
	"io"
	"net/http"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/output"
)

var gatewayNetworkCmd = &cobra.Command{
	Use:     "network",
	Aliases: []string{"net"},
	Short:   "Attach private networks to a gateway",
	Args:    cobra.NoArgs,
}

func printGatewayNetworks(w io.Writer, networks ...pidginhost.GatewayNetwork) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "NETWORK", "ROLE", "SNAT", "STATE", "ADDRESS", "PENDING", "REASON")
	for _, n := range networks {
		output.PrintRow(tw, n.Network, n.Role, n.Snat, n.State, n.Address, n.Pending, n.Reason)
	}
	tw.Flush()
}

var gatewayNetworkListCmd = &cobra.Command{
	Use:   "list <gateway-id>",
	Short: "List the networks a gateway is attached to",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		networks, err := listAll[pidginhost.GatewayNetwork]("gateway networks", func(page int32) (*pidginhost.PaginatedGatewayNetworkList, *http.Response, error) {
			return c.CloudAPI.CloudGatewaysNetworksList(cmd.Context(), ids[0]).Page(page).Execute()
		})
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), networks, func(w io.Writer) {
			printGatewayNetworks(w, networks...)
		})
	},
}

var gatewayNetworkGetCmd = &cobra.Command{
	Use:   "get <gateway-id> <network-id>",
	Short: "Get one network attachment",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		n, _, err := c.CloudAPI.CloudGatewaysNetworksRetrieve(cmd.Context(), ids[0], ids[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting gateway network", err)
		}
		if n == nil {
			return fmt.Errorf("getting gateway network: server returned no response")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), n, func(w io.Writer) {
			printGatewayNetworks(w, *n)
		})
	},
}

var gatewayNetworkAttachCmd = &cobra.Command{
	Use:   "attach <gateway-id>",
	Short: "Attach a private network to a gateway",
	Long: `Attach a private network to a gateway.

A network has at most one router: the gateway that is its servers' default
route, and the only role that can SNAT. A member is attached to the network
without routing it.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		network, _ := cmd.Flags().GetInt32("network")
		role, _ := cmd.Flags().GetString("role")
		if err := oneOf("role", role, "router", "member"); err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewGatewayNetworkCreateRequest(network, pidginhost.GatewayNetworkRoleEnum(role))
		op, _, err := c.CloudAPI.CloudGatewaysNetworksCreate(cmd.Context(), ids[0]).GatewayNetworkCreateRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("attaching network", err)
		}
		return queued(cmd, fmt.Sprintf("Attaching network %d to gateway %d", network, ids[0]), op)
	},
}

var gatewayNetworkDetachCmd = &cobra.Command{
	Use:   "detach <gateway-id> <network-id>",
	Short: "Detach a private network from a gateway",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		op, ok, err := previewed(cmd, fmt.Sprintf("Detach network %d from gateway %d?", ids[1], ids[0]),
			func(p *gatewayPreview) (*pidginhost.GatewayOperation, *http.Response, error) {
				req := c.CloudAPI.CloudGatewaysNetworksDestroy(cmd.Context(), ids[0], ids[1])
				if p != nil {
					req = req.Revision(p.Revision).Servers(p.serverIDs())
				}
				return req.Execute()
			})
		if err != nil {
			return cmdutil.APIError("detaching network", err)
		}
		if !ok {
			return nil
		}
		return queued(cmd, fmt.Sprintf("Detaching network %d from gateway %d", ids[1], ids[0]), op)
	},
}

var gatewayNetworkUpdateCmd = &cobra.Command{
	Use:   "update <gateway-id> <network-id>",
	Short: "Switch SNAT on a network the gateway routes",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		if !cmd.Flags().Changed("snat") {
			return fmt.Errorf("nothing to update: pass --snat=true or --snat=false")
		}
		snat, _ := cmd.Flags().GetBool("snat")
		c, err := client.New()
		if err != nil {
			return err
		}
		body := pidginhost.PatchedGatewayNetworkUpdateRequest{Snat: &snat}
		n, ok, err := previewed(cmd, fmt.Sprintf("Turn SNAT off for network %d on gateway %d?", ids[1], ids[0]),
			func(p *gatewayPreview) (*pidginhost.GatewayNetwork, *http.Response, error) {
				req := c.CloudAPI.CloudGatewaysNetworksPartialUpdate(cmd.Context(), ids[0], ids[1]).PatchedGatewayNetworkUpdateRequest(body)
				if p != nil {
					req = req.Revision(p.Revision).Servers(p.serverIDs())
				}
				return req.Execute()
			})
		if err != nil {
			return cmdutil.APIError("updating gateway network", err)
		}
		if !ok {
			return nil
		}
		if n == nil {
			return fmt.Errorf("updating gateway network: server returned no response")
		}
		if n.Snat != snat {
			return fmt.Errorf("updating gateway network: SNAT was not changed")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), n,
			"SNAT for network %d on gateway %d: %v (pending: %v)\n", n.Network, ids[0], n.Snat, n.Pending)
	},
}

func init() {
	gatewayNetworkAttachCmd.Flags().Int32("network", 0, "Private network ID (required)")
	gatewayNetworkAttachCmd.Flags().String("role", "router", "Role on the network: router or member")
	gatewayNetworkAttachCmd.MarkFlagRequired("network")

	gatewayNetworkUpdateCmd.Flags().Bool("snat", false, "Translate the network's outbound traffic to the gateway's address")

	gatewayNetworkCmd.AddCommand(gatewayNetworkListCmd)
	gatewayNetworkCmd.AddCommand(gatewayNetworkGetCmd)
	gatewayNetworkCmd.AddCommand(gatewayNetworkAttachCmd)
	gatewayNetworkCmd.AddCommand(gatewayNetworkDetachCmd)
	gatewayNetworkCmd.AddCommand(gatewayNetworkUpdateCmd)
}
