package compute

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/output"
)

var gatewayPortForwardCmd = &cobra.Command{
	Use:     "port-forward",
	Aliases: []string{"pf"},
	Short:   "Forward a gateway's public ports to a host on an attached network",
	Args:    cobra.NoArgs,
}

func printGatewayPortForwards(w io.Writer, forwards ...pidginhost.GatewayForward) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "ID", "PROTO", "PUBLIC PORTS", "TARGET", "NETWORK", "ALLOWED SOURCES", "ENABLED", "PENDING", "COMMENT")
	for _, f := range forwards {
		// The schema leaves allowed_sources untyped; it is a list of networks.
		sources, _ := f.AllowedSources.([]any)
		allowed := make([]string, len(sources))
		for i, s := range sources {
			allowed[i] = fmt.Sprint(s)
		}
		output.PrintRow(tw, f.Id, f.Protocol, formatPortRange(f.PublicPorts.Start, f.PublicPorts.End),
			fmt.Sprintf("%s:%d", f.Target.Address, f.Target.PortStart), f.Target.Network,
			strings.Join(allowed, ","), f.Enabled, f.Pending, f.Comment)
	}
	tw.Flush()
}

var gatewayPortForwardListCmd = &cobra.Command{
	Use:   "list <gateway-id>",
	Short: "List port forwards",
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
		forwards, err := listAll[pidginhost.GatewayForward]("gateway port forwards", func(page int32) (*pidginhost.PaginatedGatewayForwardList, *http.Response, error) {
			return c.CloudAPI.CloudGatewaysPortForwardsList(cmd.Context(), ids[0]).Page(page).Execute()
		})
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), forwards, func(w io.Writer) {
			printGatewayPortForwards(w, forwards...)
		})
	},
}

var gatewayPortForwardGetCmd = &cobra.Command{
	Use:   "get <gateway-id> <forward-id>",
	Short: "Get one port forward",
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
		forward, _, err := c.CloudAPI.CloudGatewaysPortForwardsRetrieve(cmd.Context(), ids[0], ids[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting port forward", err)
		}
		if forward == nil {
			return fmt.Errorf("getting port forward: server returned no response")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), forward, func(w io.Writer) {
			printGatewayPortForwards(w, *forward)
		})
	},
}

var gatewayPortForwardTargetFlags = []string{"target-network", "target-address", "target-port"}

func addGatewayPortForwardFlags(cmd *cobra.Command) {
	cmd.Flags().String("protocol", "", "tcp or udp")
	cmd.Flags().String("public-ports", "", "Public port or range, such as 8080 or 8080-8090")
	cmd.Flags().Int32("target-network", 0, "Attached private network the target is on")
	cmd.Flags().String("target-address", "", "The target's address on that network")
	cmd.Flags().Int32("target-port", 0, "Where the first public port goes; the next ones follow in order")
	cmd.Flags().StringSlice("allowed-sources", nil, "IPv4 networks allowed to connect (empty: any source)")
	cmd.Flags().Bool("enabled", true, "Whether the forward is active")
	cmd.Flags().String("comment", "", "A note for yourself")
}

// gatewayPortForwardBody carries only the flags that were given.
func gatewayPortForwardBody(cmd *cobra.Command) (body pidginhost.PatchedGatewayForwardWriteRequest, err error) {
	flags := cmd.Flags()
	if flags.Changed("protocol") {
		protocol, _ := flags.GetString("protocol")
		if err := oneOf("protocol", protocol, "tcp", "udp"); err != nil {
			return body, err
		}
		body.Protocol = pidginhost.GatewayForwardProtocolEnum(protocol).Ptr()
	}
	if flags.Changed("public-ports") {
		value, _ := flags.GetString("public-ports")
		ranges, err := parsePortRanges("public-ports", []string{value})
		if err != nil {
			return body, err
		}
		body.PublicPorts = &ranges[0]
	}
	given := 0
	for _, name := range gatewayPortForwardTargetFlags {
		if flags.Changed(name) {
			given++
		}
	}
	if given > 0 {
		// The API replaces a target whole, so part of one would blank the rest.
		if given != len(gatewayPortForwardTargetFlags) {
			return body, fmt.Errorf("a target needs --target-network, --target-address and --target-port together")
		}
		network, _ := flags.GetInt32("target-network")
		address, _ := flags.GetString("target-address")
		port, _ := flags.GetInt32("target-port")
		if port < 1 || port > 65535 {
			return body, fmt.Errorf("--target-port must be between 1 and 65535, got %d", port)
		}
		body.Target = pidginhost.NewGatewayForwardTargetRequest(network, address, port)
	}
	if flags.Changed("allowed-sources") {
		sources, _ := flags.GetStringSlice("allowed-sources")
		body.AllowedSources = append([]string{}, sources...) // non-nil, so an empty list is sent
	}
	if flags.Changed("enabled") {
		enabled, _ := flags.GetBool("enabled")
		body.Enabled = &enabled
	}
	if flags.Changed("comment") {
		comment, _ := flags.GetString("comment")
		body.Comment = &comment
	}
	return body, nil
}

var gatewayPortForwardCreateCmd = &cobra.Command{
	Use:   "create <gateway-id>",
	Short: "Add a port forward",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		patch, err := gatewayPortForwardBody(cmd)
		if err != nil {
			return err
		}
		body, err := asCreate[pidginhost.GatewayForwardWriteRequest](patch)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		forward, _, err := c.CloudAPI.CloudGatewaysPortForwardsCreate(cmd.Context(), ids[0]).GatewayForwardWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating port forward", err)
		}
		if forward == nil {
			return fmt.Errorf("creating port forward: server returned no response")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), forward,
			"Port forward created (ID: %d, pending: %v)\n", forward.Id, forward.Pending)
	},
}

var gatewayPortForwardUpdateCmd = &cobra.Command{
	Use:   "update <gateway-id> <forward-id>",
	Short: "Change a port forward",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		body, err := gatewayPortForwardBody(cmd)
		if err != nil {
			return err
		}
		if err := requireChange(body); err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		forward, _, err := c.CloudAPI.CloudGatewaysPortForwardsPartialUpdate(cmd.Context(), ids[0], ids[1]).PatchedGatewayForwardWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating port forward", err)
		}
		if forward == nil {
			return fmt.Errorf("updating port forward: server returned no response")
		}
		if err := checkApplied("updating port forward", body, forward); err != nil {
			return err
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), forward,
			"Port forward %d updated (pending: %v)\n", forward.Id, forward.Pending)
	},
}

var gatewayPortForwardDeleteCmd = &cobra.Command{
	Use:     "delete <gateway-id> <forward-id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a port forward",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		op, ok, err := previewed(cmd, fmt.Sprintf("Delete port forward %d of gateway %d?", ids[1], ids[0]),
			func(p *gatewayPreview) (*pidginhost.GatewayOperation, *http.Response, error) {
				req := c.CloudAPI.CloudGatewaysPortForwardsDestroy(cmd.Context(), ids[0], ids[1])
				if p != nil {
					req = req.Revision(p.Revision)
				}
				return req.Execute()
			})
		if err != nil {
			return cmdutil.APIError("deleting port forward", err)
		}
		if !ok {
			return nil
		}
		return queued(cmd, fmt.Sprintf("Deleting port forward %d", ids[1]), op)
	},
}

func init() {
	addGatewayPortForwardFlags(gatewayPortForwardCreateCmd)
	for _, name := range append([]string{"protocol", "public-ports"}, gatewayPortForwardTargetFlags...) {
		gatewayPortForwardCreateCmd.MarkFlagRequired(name)
	}
	addGatewayPortForwardFlags(gatewayPortForwardUpdateCmd)

	gatewayPortForwardCmd.AddCommand(gatewayPortForwardListCmd)
	gatewayPortForwardCmd.AddCommand(gatewayPortForwardGetCmd)
	gatewayPortForwardCmd.AddCommand(gatewayPortForwardCreateCmd)
	gatewayPortForwardCmd.AddCommand(gatewayPortForwardUpdateCmd)
	gatewayPortForwardCmd.AddCommand(gatewayPortForwardDeleteCmd)
	gatewayCmd.AddCommand(gatewayPortForwardCmd)
}
