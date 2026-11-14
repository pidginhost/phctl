package compute

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/output"
)

// The load balancer: a listener takes a public port and sends it to a pool of
// backends, directly (tcp, udp) or through routes that pick a pool by host
// and path (http).

// The given* helpers return a flag's value only if it was passed, which is
// what a PATCH body needs: nil means "leave it alone".
func givenString(flags *pflag.FlagSet, name string) *string {
	if !flags.Changed(name) {
		return nil
	}
	v, _ := flags.GetString(name)
	return &v
}

func givenInt32(flags *pflag.FlagSet, name string) *int32 {
	if !flags.Changed(name) {
		return nil
	}
	v, _ := flags.GetInt32(name)
	return &v
}

func givenBool(flags *pflag.FlagSet, name string) *bool {
	if !flags.Changed(name) {
		return nil
	}
	v, _ := flags.GetBool(name)
	return &v
}

// givenList returns a non-nil slice, so an empty list is sent and clears.
func givenList(flags *pflag.FlagSet, name string) []string {
	if !flags.Changed(name) {
		return nil
	}
	v, _ := flags.GetStringSlice(name)
	return append([]string{}, v...)
}

// givenChoice is givenString for a flag that takes one of a fixed set.
func givenChoice(flags *pflag.FlagSet, name string, allowed ...string) (*string, error) {
	v := givenString(flags, name)
	if v == nil {
		return nil, nil
	}
	return v, oneOf(name, *v, allowed...)
}

// --- Backend pools ---

var gatewayPoolCmd = &cobra.Command{
	Use:   "pool",
	Short: "Manage a gateway's backend pools",
	Args:  cobra.NoArgs,
}

func printGatewayPools(w io.Writer, pools ...pidginhost.GatewayBackendPool) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "ID", "NAME", "POLICY", "PROXY PROTOCOL", "HEALTH CHECK", "PENDING")
	for _, p := range pools {
		check := string(p.HealthCheck.Type)
		if path := p.HealthCheck.Path.Get(); path != nil {
			check += " " + *path
		}
		output.PrintRow(tw, p.Id, p.Name, p.Policy, p.ProxyProtocol, check, p.Pending)
	}
	tw.Flush()
}

var gatewayPoolListCmd = &cobra.Command{
	Use:   "list <gateway-id>",
	Short: "List backend pools",
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
		pools, err := listAll[pidginhost.GatewayBackendPool]("gateway pools", func(page int32) (*pidginhost.PaginatedGatewayBackendPoolList, *http.Response, error) {
			return c.CloudAPI.CloudGatewaysPoolsList(cmd.Context(), ids[0]).Page(page).Execute()
		})
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), pools, func(w io.Writer) {
			printGatewayPools(w, pools...)
		})
	},
}

var gatewayPoolGetCmd = &cobra.Command{
	Use:   "get <gateway-id> <pool-id>",
	Short: "Get one backend pool",
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
		pool, _, err := c.CloudAPI.CloudGatewaysPoolsRetrieve(cmd.Context(), ids[0], ids[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting pool", err)
		}
		if pool == nil {
			return fmt.Errorf("getting pool: server returned no response")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), pool, func(w io.Writer) {
			check := pool.HealthCheck
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", pool.Id)
			output.PrintRow(tw, "Name:", pool.Name)
			output.PrintRow(tw, "Policy:", pool.Policy)
			output.PrintRow(tw, "PROXY Protocol:", pool.ProxyProtocol)
			output.PrintRow(tw, "Pending:", pool.Pending)
			output.PrintRow(tw, "Health Check:", check.Type)
			output.PrintRow(tw, "  Path:", output.Pstr(check.Path.Get()))
			output.PrintRow(tw, "  Status:", fmt.Sprintf("%s to %s", output.Pstr(check.StatusMin.Get()), output.Pstr(check.StatusMax.Get())))
			output.PrintRow(tw, "  Interval (s):", output.Pstr(check.IntervalS.Get()))
			output.PrintRow(tw, "  Timeout (s):", output.Pstr(check.TimeoutS.Get()))
			output.PrintRow(tw, "  Healthy After:", output.Pstr(check.HealthyThreshold.Get()))
			output.PrintRow(tw, "  Unhealthy After:", output.Pstr(check.UnhealthyThreshold.Get()))
			output.PrintRow(tw, "  Port:", output.Pstr(check.Port.Get()))
			tw.Flush()
		})
	},
}

func addGatewayPoolFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "Pool name")
	cmd.Flags().String("policy", "", "How servers are picked: round_robin or least_request")
	cmd.Flags().Bool("proxy-protocol", false, "Send PROXY protocol v2 to the servers (tcp only)")
	cmd.Flags().String("health-check", "", "Health check type: none, tcp or http")
	cmd.Flags().String("health-path", "", "Path to request (http only)")
	cmd.Flags().Int32("health-status-min", 0, "Lowest healthy status code (http only)")
	cmd.Flags().Int32("health-status-max", 0, "Highest healthy status code (http only)")
	cmd.Flags().Int32("health-interval", 0, "Seconds between checks")
	cmd.Flags().Int32("health-timeout", 0, "Seconds to wait for an answer; below the interval")
	cmd.Flags().Int32("healthy-threshold", 0, "Passes in a row before a server is healthy")
	cmd.Flags().Int32("unhealthy-threshold", 0, "Failures in a row before a server is unhealthy")
	cmd.Flags().Int32("health-port", 0, "Port to check (0: each server's own port)")
}

// gatewayPoolBody carries only the flags that were given.
func gatewayPoolBody(cmd *cobra.Command) (body pidginhost.PatchedGatewayBackendPoolWriteRequest, err error) {
	flags := cmd.Flags()
	body.Name = givenString(flags, "name")
	policy, err := givenChoice(flags, "policy", "round_robin", "least_request")
	if err != nil {
		return body, err
	}
	if policy != nil {
		body.Policy = pidginhost.GatewayBackendPoolPolicyEnum(*policy).Ptr()
	}
	body.ProxyProtocol = givenBool(flags, "proxy-protocol")

	checkType, err := givenChoice(flags, "health-check", "none", "tcp", "http")
	if err != nil {
		return body, err
	}
	check := pidginhost.GatewayHealthCheckWriteRequest{
		Path:               givenString(flags, "health-path"),
		StatusMin:          givenInt32(flags, "health-status-min"),
		StatusMax:          givenInt32(flags, "health-status-max"),
		IntervalS:          givenInt32(flags, "health-interval"),
		TimeoutS:           givenInt32(flags, "health-timeout"),
		HealthyThreshold:   givenInt32(flags, "healthy-threshold"),
		UnhealthyThreshold: givenInt32(flags, "unhealthy-threshold"),
	}
	if checkType != nil {
		check.Type = pidginhost.GatewayHealthCheckTypeEnum(*checkType).Ptr()
	}
	if port := givenInt32(flags, "health-port"); port != nil {
		if *port == 0 {
			check.SetPortNil()
		} else {
			check.SetPort(*port)
		}
	}
	// The API takes just the check fields to change, so send the object only
	// when one was given.
	if requireChange(check) == nil {
		body.HealthCheck = &check
	}
	return body, nil
}

var gatewayPoolCreateCmd = &cobra.Command{
	Use:   "create <gateway-id>",
	Short: "Add a backend pool",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		patch, err := gatewayPoolBody(cmd)
		if err != nil {
			return err
		}
		body, err := asCreate[pidginhost.GatewayBackendPoolWriteRequest](patch)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		pool, _, err := c.CloudAPI.CloudGatewaysPoolsCreate(cmd.Context(), ids[0]).GatewayBackendPoolWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating pool", err)
		}
		if pool == nil {
			return fmt.Errorf("creating pool: server returned no response")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), pool,
			"Pool %s created (ID: %d, pending: %v)\n", pool.Name, pool.Id, pool.Pending)
	},
}

var gatewayPoolUpdateCmd = &cobra.Command{
	Use:   "update <gateway-id> <pool-id>",
	Short: "Change a backend pool",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		body, err := gatewayPoolBody(cmd)
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
		pool, _, err := c.CloudAPI.CloudGatewaysPoolsPartialUpdate(cmd.Context(), ids[0], ids[1]).PatchedGatewayBackendPoolWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating pool", err)
		}
		if pool == nil {
			return fmt.Errorf("updating pool: server returned no response")
		}
		if err := checkApplied("updating pool", body, pool); err != nil {
			return err
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), pool,
			"Pool %d updated (pending: %v)\n", pool.Id, pool.Pending)
	},
}

var gatewayPoolDeleteCmd = &cobra.Command{
	Use:     "delete <gateway-id> <pool-id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete an unused backend pool and its servers",
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
		op, ok, err := previewed(cmd, fmt.Sprintf("Delete pool %d of gateway %d?", ids[1], ids[0]),
			func(p *gatewayPreview) (*pidginhost.GatewayOperation, *http.Response, error) {
				req := c.CloudAPI.CloudGatewaysPoolsDestroy(cmd.Context(), ids[0], ids[1])
				if p != nil {
					req = req.Revision(p.Revision)
				}
				return req.Execute()
			})
		if err != nil {
			return cmdutil.APIError("deleting pool", err)
		}
		if !ok {
			return nil
		}
		return queued(cmd, fmt.Sprintf("Deleting pool %d", ids[1]), op)
	},
}

// --- Backends: a pool's servers ---
//
// The SDK takes these IDs as (gateway, backend, pool), not in path order.

var gatewayBackendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Manage the servers of a backend pool",
	Args:  cobra.NoArgs,
}

func printGatewayBackends(w io.Writer, backends ...pidginhost.GatewayBackend) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "ID", "NETWORK", "ADDRESS", "PORT", "WEIGHT", "ENABLED", "HEALTH", "PENDING")
	for _, b := range backends {
		output.PrintRow(tw, b.Id, b.Network, b.Address, b.Port, b.Weight, b.Enabled, b.Health, b.Pending)
	}
	tw.Flush()
}

var gatewayBackendListCmd = &cobra.Command{
	Use:   "list <gateway-id> <pool-id>",
	Short: "List a pool's servers and their health",
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
		backends, err := listAll[pidginhost.GatewayBackend]("pool backends", func(page int32) (*pidginhost.PaginatedGatewayBackendList, *http.Response, error) {
			return c.CloudAPI.CloudGatewaysPoolsBackendsList(cmd.Context(), ids[0], ids[1]).Page(page).Execute()
		})
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), backends, func(w io.Writer) {
			printGatewayBackends(w, backends...)
		})
	},
}

var gatewayBackendGetCmd = &cobra.Command{
	Use:   "get <gateway-id> <pool-id> <backend-id>",
	Short: "Get one server of a pool",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		backend, _, err := c.CloudAPI.CloudGatewaysPoolsBackendsRetrieve(cmd.Context(), ids[0], ids[2], ids[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting backend", err)
		}
		if backend == nil {
			return fmt.Errorf("getting backend: server returned no response")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), backend, func(w io.Writer) {
			printGatewayBackends(w, *backend)
		})
	},
}

func addGatewayBackendFlags(cmd *cobra.Command) {
	cmd.Flags().Int32("network", 0, "Attached private network the server is on")
	cmd.Flags().String("address", "", "The server's address on that network")
	cmd.Flags().Int32("port", 0, "Port the server listens on")
	cmd.Flags().Int32("weight", 0, "Share of the traffic, 1 to 100")
	cmd.Flags().Bool("enabled", true, "Whether the server receives traffic")
}

// gatewayBackendBody carries only the flags that were given.
func gatewayBackendBody(cmd *cobra.Command) pidginhost.PatchedGatewayBackendWriteRequest {
	flags := cmd.Flags()
	return pidginhost.PatchedGatewayBackendWriteRequest{
		Network: givenInt32(flags, "network"),
		Address: givenString(flags, "address"),
		Port:    givenInt32(flags, "port"),
		Weight:  givenInt32(flags, "weight"),
		Enabled: givenBool(flags, "enabled"),
	}
}

var gatewayBackendCreateCmd = &cobra.Command{
	Use:   "create <gateway-id> <pool-id>",
	Short: "Add a server to a pool",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		body, err := asCreate[pidginhost.GatewayBackendWriteRequest](gatewayBackendBody(cmd))
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		backend, _, err := c.CloudAPI.CloudGatewaysPoolsBackendsCreate(cmd.Context(), ids[0], ids[1]).GatewayBackendWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating backend", err)
		}
		if backend == nil {
			return fmt.Errorf("creating backend: server returned no response")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), backend,
			"Backend %s:%d added (ID: %d, pending: %v)\n", backend.Address, backend.Port, backend.Id, backend.Pending)
	},
}

var gatewayBackendUpdateCmd = &cobra.Command{
	Use:   "update <gateway-id> <pool-id> <backend-id>",
	Short: "Change a server of a pool",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		body := gatewayBackendBody(cmd)
		if err := requireChange(body); err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		backend, _, err := c.CloudAPI.CloudGatewaysPoolsBackendsPartialUpdate(cmd.Context(), ids[0], ids[2], ids[1]).PatchedGatewayBackendWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating backend", err)
		}
		if backend == nil {
			return fmt.Errorf("updating backend: server returned no response")
		}
		if err := checkApplied("updating backend", body, backend); err != nil {
			return err
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), backend,
			"Backend %d updated (pending: %v)\n", backend.Id, backend.Pending)
	},
}

var gatewayBackendDeleteCmd = &cobra.Command{
	Use:     "delete <gateway-id> <pool-id> <backend-id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Remove a server from a pool",
	Args:    cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		op, ok, err := previewed(cmd, fmt.Sprintf("Remove backend %d from pool %d?", ids[2], ids[1]),
			func(p *gatewayPreview) (*pidginhost.GatewayOperation, *http.Response, error) {
				req := c.CloudAPI.CloudGatewaysPoolsBackendsDestroy(cmd.Context(), ids[0], ids[2], ids[1])
				if p != nil {
					req = req.Revision(p.Revision)
				}
				return req.Execute()
			})
		if err != nil {
			return cmdutil.APIError("deleting backend", err)
		}
		if !ok {
			return nil
		}
		return queued(cmd, fmt.Sprintf("Removing backend %d", ids[2]), op)
	},
}

// --- Listeners ---

var gatewayListenerCmd = &cobra.Command{
	Use:   "listener",
	Short: "Manage a gateway's load balancer listeners",
	Long: `Manage a gateway's load balancer listeners: one public port each.

A tcp or udp listener sends everything to one pool. An http listener has no
pool of its own; its routes pick one by host and path ('gateway route').
Protocol, port and mode are fixed once the listener exists.`,
	Args: cobra.NoArgs,
}

func printGatewayListeners(w io.Writer, listeners ...pidginhost.GatewayLBListener) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "ID", "PROTO", "PORT", "MODE", "POOL", "ALLOWED SOURCES", "ENABLED", "PENDING", "COMMENT")
	for _, l := range listeners {
		output.PrintRow(tw, l.Id, l.Protocol, l.Port, l.Mode, output.Pstr(l.Pool.Get()),
			strings.Join(l.AllowedSources, ","), l.Enabled, l.Pending, l.Comment)
	}
	tw.Flush()
}

var gatewayListenerListCmd = &cobra.Command{
	Use:   "list <gateway-id>",
	Short: "List listeners",
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
		listeners, err := listAll[pidginhost.GatewayLBListener]("gateway listeners", func(page int32) (*pidginhost.PaginatedGatewayLBListenerList, *http.Response, error) {
			return c.CloudAPI.CloudGatewaysListenersList(cmd.Context(), ids[0]).Page(page).Execute()
		})
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), listeners, func(w io.Writer) {
			printGatewayListeners(w, listeners...)
		})
	},
}

var gatewayListenerGetCmd = &cobra.Command{
	Use:   "get <gateway-id> <listener-id>",
	Short: "Get one listener",
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
		listener, _, err := c.CloudAPI.CloudGatewaysListenersRetrieve(cmd.Context(), ids[0], ids[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting listener", err)
		}
		if listener == nil {
			return fmt.Errorf("getting listener: server returned no response")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), listener, func(w io.Writer) {
			printGatewayListeners(w, *listener)
		})
	},
}

func addGatewayListenerFlags(cmd *cobra.Command, create bool) {
	if create {
		cmd.Flags().String("protocol", "", "tcp or udp")
		cmd.Flags().Int32("port", 0, "Public port")
		cmd.Flags().String("mode", "", "tcp, udp, http or https")
	}
	cmd.Flags().Int32("pool", 0, "Backend pool to send to (tcp and udp listeners)")
	cmd.Flags().StringSlice("allowed-sources", nil, "Networks allowed to connect (empty: any source)")
	cmd.Flags().Bool("enabled", true, "Whether the listener is active")
	cmd.Flags().String("comment", "", "A note for yourself")
}

// gatewayListenerBody carries only the flags that were given.
func gatewayListenerBody(cmd *cobra.Command) (body pidginhost.PatchedGatewayLBListenerWriteRequest, err error) {
	flags := cmd.Flags()
	if flags.Lookup("protocol") != nil { // create only: these are fixed afterwards
		protocol, err := givenChoice(flags, "protocol", "tcp", "udp")
		if err != nil {
			return body, err
		}
		if protocol != nil {
			body.Protocol = pidginhost.GatewayForwardProtocolEnum(*protocol).Ptr()
		}
		mode, err := givenChoice(flags, "mode", "tcp", "udp", "http", "https")
		if err != nil {
			return body, err
		}
		if mode != nil {
			body.Mode = pidginhost.GatewayLBListenerModeEnum(*mode).Ptr()
		}
		body.Port = givenInt32(flags, "port")
	}
	if pool := givenInt32(flags, "pool"); pool != nil {
		body.SetPool(*pool)
	}
	body.AllowedSources = givenList(flags, "allowed-sources")
	body.Enabled = givenBool(flags, "enabled")
	body.Comment = givenString(flags, "comment")
	return body, nil
}

var gatewayListenerCreateCmd = &cobra.Command{
	Use:   "create <gateway-id>",
	Short: "Add a listener",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		patch, err := gatewayListenerBody(cmd)
		if err != nil {
			return err
		}
		body, err := asCreate[pidginhost.GatewayLBListenerWriteRequest](patch)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		listener, _, err := c.CloudAPI.CloudGatewaysListenersCreate(cmd.Context(), ids[0]).GatewayLBListenerWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating listener", err)
		}
		if listener == nil {
			return fmt.Errorf("creating listener: server returned no response")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), listener,
			"Listener created on port %d (ID: %d, mode: %s, pending: %v)\n", listener.Port, listener.Id, listener.Mode, listener.Pending)
	},
}

var gatewayListenerUpdateCmd = &cobra.Command{
	Use:   "update <gateway-id> <listener-id>",
	Short: "Change a listener's pool, sources, comment or enabled flag",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		body, err := gatewayListenerBody(cmd)
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
		listener, _, err := c.CloudAPI.CloudGatewaysListenersPartialUpdate(cmd.Context(), ids[0], ids[1]).PatchedGatewayLBListenerWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating listener", err)
		}
		if listener == nil {
			return fmt.Errorf("updating listener: server returned no response")
		}
		if err := checkApplied("updating listener", body, listener); err != nil {
			return err
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), listener,
			"Listener %d updated (pending: %v)\n", listener.Id, listener.Pending)
	},
}

var gatewayListenerDeleteCmd = &cobra.Command{
	Use:     "delete <gateway-id> <listener-id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a listener and its routes",
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
		op, ok, err := previewed(cmd, fmt.Sprintf("Delete listener %d of gateway %d and its routes?", ids[1], ids[0]),
			func(p *gatewayPreview) (*pidginhost.GatewayOperation, *http.Response, error) {
				req := c.CloudAPI.CloudGatewaysListenersDestroy(cmd.Context(), ids[0], ids[1])
				if p != nil {
					req = req.Revision(p.Revision)
				}
				return req.Execute()
			})
		if err != nil {
			return cmdutil.APIError("deleting listener", err)
		}
		if !ok {
			return nil
		}
		return queued(cmd, fmt.Sprintf("Deleting listener %d", ids[1]), op)
	},
}

// --- Routes: an http listener's host and path rules ---
//
// The SDK takes these IDs as (gateway, route, listener), not in path order.

var gatewayRouteCmd = &cobra.Command{
	Use:   "route",
	Short: "Manage the routes of an http listener",
	Args:  cobra.NoArgs,
}

func printGatewayRoutes(w io.Writer, routes ...pidginhost.GatewayRoute) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "ID", "HOSTNAMES", "PATH PREFIX", "POOL", "ENABLED", "PENDING")
	for _, r := range routes {
		hostnames := "<any>"
		if len(r.Hostnames) > 0 {
			hostnames = strings.Join(r.Hostnames, ",")
		}
		output.PrintRow(tw, r.Id, hostnames, r.PathPrefix, r.Pool, r.Enabled, r.Pending)
	}
	tw.Flush()
}

var gatewayRouteListCmd = &cobra.Command{
	Use:   "list <gateway-id> <listener-id>",
	Short: "List a listener's routes",
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
		routes, err := listAll[pidginhost.GatewayRoute]("listener routes", func(page int32) (*pidginhost.PaginatedGatewayRouteList, *http.Response, error) {
			return c.CloudAPI.CloudGatewaysListenersRoutesList(cmd.Context(), ids[0], ids[1]).Page(page).Execute()
		})
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), routes, func(w io.Writer) {
			printGatewayRoutes(w, routes...)
		})
	},
}

var gatewayRouteGetCmd = &cobra.Command{
	Use:   "get <gateway-id> <listener-id> <route-id>",
	Short: "Get one route",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		route, _, err := c.CloudAPI.CloudGatewaysListenersRoutesRetrieve(cmd.Context(), ids[0], ids[2], ids[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting route", err)
		}
		if route == nil {
			return fmt.Errorf("getting route: server returned no response")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route, func(w io.Writer) {
			printGatewayRoutes(w, *route)
		})
	},
}

func addGatewayRouteFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("hostnames", nil, "Host names to match (empty: every host)")
	cmd.Flags().String("path-prefix", "", "Path prefix to match, literal and case-sensitive")
	cmd.Flags().Int32("pool", 0, "Backend pool to send to")
	cmd.Flags().Bool("enabled", true, "Whether the route is active")
}

// gatewayRouteBody carries only the flags that were given.
func gatewayRouteBody(cmd *cobra.Command) pidginhost.PatchedGatewayRouteWriteRequest {
	flags := cmd.Flags()
	return pidginhost.PatchedGatewayRouteWriteRequest{
		Hostnames:  givenList(flags, "hostnames"),
		PathPrefix: givenString(flags, "path-prefix"),
		Pool:       givenInt32(flags, "pool"),
		Enabled:    givenBool(flags, "enabled"),
	}
}

var gatewayRouteCreateCmd = &cobra.Command{
	Use:   "create <gateway-id> <listener-id>",
	Short: "Add a route to an http listener",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		body, err := asCreate[pidginhost.GatewayRouteWriteRequest](gatewayRouteBody(cmd))
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		route, _, err := c.CloudAPI.CloudGatewaysListenersRoutesCreate(cmd.Context(), ids[0], ids[1]).GatewayRouteWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating route", err)
		}
		if route == nil {
			return fmt.Errorf("creating route: server returned no response")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route,
			"Route created (ID: %d, pool: %d, pending: %v)\n", route.Id, route.Pool, route.Pending)
	},
}

var gatewayRouteUpdateCmd = &cobra.Command{
	Use:   "update <gateway-id> <listener-id> <route-id>",
	Short: "Change a route",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		body := gatewayRouteBody(cmd)
		if err := requireChange(body); err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		route, _, err := c.CloudAPI.CloudGatewaysListenersRoutesPartialUpdate(cmd.Context(), ids[0], ids[2], ids[1]).PatchedGatewayRouteWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating route", err)
		}
		if route == nil {
			return fmt.Errorf("updating route: server returned no response")
		}
		if err := checkApplied("updating route", body, route); err != nil {
			return err
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route,
			"Route %d updated (pending: %v)\n", route.Id, route.Pending)
	},
}

var gatewayRouteDeleteCmd = &cobra.Command{
	Use:     "delete <gateway-id> <listener-id> <route-id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a route",
	Args:    cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		op, ok, err := previewed(cmd, fmt.Sprintf("Delete route %d of listener %d?", ids[2], ids[1]),
			func(p *gatewayPreview) (*pidginhost.GatewayOperation, *http.Response, error) {
				req := c.CloudAPI.CloudGatewaysListenersRoutesDestroy(cmd.Context(), ids[0], ids[2], ids[1])
				if p != nil {
					req = req.Revision(p.Revision)
				}
				return req.Execute()
			})
		if err != nil {
			return cmdutil.APIError("deleting route", err)
		}
		if !ok {
			return nil
		}
		return queued(cmd, fmt.Sprintf("Deleting route %d", ids[2]), op)
	},
}

func init() {
	addGatewayPoolFlags(gatewayPoolCreateCmd)
	gatewayPoolCreateCmd.MarkFlagRequired("name")
	addGatewayPoolFlags(gatewayPoolUpdateCmd)

	addGatewayBackendFlags(gatewayBackendCreateCmd)
	for _, name := range []string{"network", "address", "port"} {
		gatewayBackendCreateCmd.MarkFlagRequired(name)
	}
	addGatewayBackendFlags(gatewayBackendUpdateCmd)

	addGatewayListenerFlags(gatewayListenerCreateCmd, true)
	for _, name := range []string{"protocol", "port", "mode"} {
		gatewayListenerCreateCmd.MarkFlagRequired(name)
	}
	addGatewayListenerFlags(gatewayListenerUpdateCmd, false)

	addGatewayRouteFlags(gatewayRouteCreateCmd)
	gatewayRouteCreateCmd.MarkFlagRequired("pool")
	addGatewayRouteFlags(gatewayRouteUpdateCmd)

	for parent, children := range map[*cobra.Command][]*cobra.Command{
		gatewayPoolCmd:     {gatewayPoolListCmd, gatewayPoolGetCmd, gatewayPoolCreateCmd, gatewayPoolUpdateCmd, gatewayPoolDeleteCmd},
		gatewayBackendCmd:  {gatewayBackendListCmd, gatewayBackendGetCmd, gatewayBackendCreateCmd, gatewayBackendUpdateCmd, gatewayBackendDeleteCmd},
		gatewayListenerCmd: {gatewayListenerListCmd, gatewayListenerGetCmd, gatewayListenerCreateCmd, gatewayListenerUpdateCmd, gatewayListenerDeleteCmd},
		gatewayRouteCmd:    {gatewayRouteListCmd, gatewayRouteGetCmd, gatewayRouteCreateCmd, gatewayRouteUpdateCmd, gatewayRouteDeleteCmd},
	} {
		parent.AddCommand(children...)
		gatewayCmd.AddCommand(parent)
	}
}
