package compute

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var gatewayCmd = &cobra.Command{
	Use:     "gateway",
	Aliases: []string{"gw"},
	Short:   "Manage gateways",
	Long: `Manage gateways: routers that give private networks a way out, with a
firewall, port forwards and a load balancer.

Changes that take time answer with an operation; follow it with
'gateway operation get'. A change that would cut servers off is previewed
first and applied only after you confirm it (or pass --force).`,
	Args: cobra.NoArgs,
}

// gatewayPreview is what the API answers a risky change with instead of
// applying it: the revision the preview was computed at and what the change
// would cut off. Sent back unchanged, it confirms the change.
type gatewayPreview struct {
	Revision int32
	Servers  []pidginhost.GatewayPreviewServer
	Networks []pidginhost.GatewayPreviewNetwork
}

func (p gatewayPreview) serverIDs() []int32 {
	ids := make([]int32, len(p.Servers))
	for i, s := range p.Servers {
		ids[i] = s.Id
	}
	return ids
}

func (p gatewayPreview) networkIDs() []int32 {
	ids := make([]int32, len(p.Networks))
	for i, n := range p.Networks {
		ids[i] = n.Id
	}
	return ids
}

// previewOf extracts the preview from a 409. A 409 without one is a refusal
// (another operation is running, the object is in use), not a question.
func previewOf(err error) (gatewayPreview, bool) {
	var apiErr *pidginhost.GenericOpenAPIError
	if !errors.As(err, &apiErr) {
		return gatewayPreview{}, false
	}
	switch m := apiErr.Model().(type) {
	case pidginhost.GatewayConflict:
		if p := m.Extra.Preview; p != nil {
			return gatewayPreview{Revision: p.Revision, Servers: p.Servers}, true
		}
	case pidginhost.GatewayDeleteConflict:
		return deletePreviewOf(m.Extra)
	case pidginhost.GatewayReenrollConflict:
		return deletePreviewOf(m.Extra)
	}
	return gatewayPreview{}, false
}

func deletePreviewOf(extra pidginhost.GatewayDeleteConflictExtra) (gatewayPreview, bool) {
	p := extra.Preview
	if p == nil {
		return gatewayPreview{}, false
	}
	return gatewayPreview{Revision: p.Revision, Servers: p.Servers, Networks: p.Networks}, true
}

// previewed runs a change the API previews before applying. The first call
// carries no preview; the API never applies such a request (no gateway has
// revision 0) and answers 409 with what the change would cut off. Once the
// operator agrees, the same call goes out again carrying the preview back.
//
// ok is false when the operator declined. A change the API applies without a
// preview (turning SNAT on) returns from the first call with no prompt.
func previewed[T any](cmd *cobra.Command, question string,
	call func(p *gatewayPreview) (*T, *http.Response, error)) (resp *T, ok bool, err error) {
	resp, _, err = call(nil)
	p, isPreview := previewOf(err)
	if !isPreview {
		return resp, err == nil, err
	}
	if !cmdutil.Force(cmd) {
		w := cmd.ErrOrStderr()
		for _, s := range p.Servers {
			fmt.Fprintf(w, "Server %s (%d) will be cut off.\n", s.Hostname, s.Id)
		}
		for _, n := range p.Networks {
			fmt.Fprintf(w, "Network %s (%d) will be detached.\n", n.Name, n.Id)
		}
		if !confirm.Action(cmd.InOrStdin(), w, question) {
			return nil, false, nil
		}
	}
	resp, _, err = call(&p)
	return resp, err == nil, err
}

// queued reports an operation the API accepted but has not finished.
func queued(cmd *cobra.Command, what string, op *pidginhost.GatewayOperation) error {
	if op == nil {
		return fmt.Errorf("%s: server returned no response", what)
	}
	return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), op,
		"%s: operation %s is %s.\n", what, op.Id, op.Status)
}

// listAll collects every page of an SDK list call.
func listAll[T any, P interface {
	comparable
	GetResults() []T
	GetNext() string
}](what string, fetch func(page int32) (P, *http.Response, error)) ([]T, error) {
	items, err := cmdutil.FetchAll(func(page int32) ([]T, bool, error) {
		resp, _, err := fetch(page)
		if err != nil {
			return nil, false, err
		}
		var none P
		if resp == none {
			return nil, false, errors.New("server returned no response page")
		}
		return resp.GetResults(), resp.GetNext() != "", nil
	})
	if err != nil {
		return nil, cmdutil.APIError("listing "+what, err)
	}
	return items, nil
}

// parseIDs parses the leading positional arguments, which are all numeric IDs.
func parseIDs(args []string) ([]int32, error) {
	ids := make([]int32, len(args))
	for i, arg := range args {
		id, err := cmdutil.ParseInt32(arg)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return ids, nil
}

func oneOf(flag, value string, allowed ...string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("--%s must be one of %s, got %q", flag, strings.Join(allowed, ", "), value)
}

var gatewayListCmd = &cobra.Command{
	Use:   "list",
	Short: "List gateways",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		gateways, err := listAll[pidginhost.Gateway]("gateways", func(page int32) (*pidginhost.PaginatedGatewayList, *http.Response, error) {
			return c.CloudAPI.CloudGatewaysList(cmd.Context()).Page(page).Execute()
		})
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), gateways, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "STATE", "IPV4", "IPV6", "HA")
			for _, g := range gateways {
				output.PrintRow(tw, g.Id, g.Name, g.State, output.Pstr(g.Ipv4.Get()), output.Pstr(g.Ipv6.Get()), g.HaEnabled)
			}
			tw.Flush()
		})
	},
}

func printGateway(w io.Writer, g *pidginhost.Gateway) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "ID:", g.Id)
	output.PrintRow(tw, "Name:", g.Name)
	output.PrintRow(tw, "State:", g.State)
	if g.StateReason != "" {
		output.PrintRow(tw, "Reason:", g.StateReason)
	}
	output.PrintRow(tw, "Status:", g.Status)
	output.PrintRow(tw, "IPv4:", output.Pstr(g.Ipv4.Get()))
	output.PrintRow(tw, "IPv6:", output.Pstr(g.Ipv6.Get()))
	output.PrintRow(tw, "Revision:", fmt.Sprintf("%s applied, %d desired", output.Pstr(g.AppliedRevision.Get()), g.DesiredRevision))
	output.PrintRow(tw, "Maintenance:", fmt.Sprintf("weekday %d, hour %d", g.MaintenanceWeekday, g.MaintenanceHour))
	output.PrintRow(tw, "HA Enabled:", g.HaEnabled)
	output.PrintRow(tw, "Operation:", output.Pstr(g.CurrentOperation.Get()))
	output.PrintRow(tw, "Created:", g.Created)
	tw.Flush()
}

var gatewayGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get gateway details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := parseIDs(args); err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		g, _, err := c.CloudAPI.CloudGatewaysRetrieve(cmd.Context(), args[0]).Execute()
		if err != nil {
			return cmdutil.APIError("getting gateway", err)
		}
		if g == nil {
			return fmt.Errorf("getting gateway: server returned no response")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), g, func(w io.Writer) { printGateway(w, g) })
	},
}

var gatewayCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a gateway",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		ipv4, _ := cmd.Flags().GetString("ipv4")
		ipv6, _ := cmd.Flags().GetBool("ipv6")
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Create gateway %q? A gateway is billed for as long as it exists.", name)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewGatewayCreateRequest(name)
		if ipv4 != "" {
			body.SetIpv4(ipv4)
		}
		if ipv6 {
			body.SetIpv6(true)
		}
		g, _, err := c.CloudAPI.CloudGatewaysCreate(cmd.Context()).GatewayCreateRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating gateway", err)
		}
		if g == nil {
			return fmt.Errorf("creating gateway: server returned no response")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), g,
			"Gateway %s is being created (ID: %d, state: %s)\n", g.Name, g.Id, g.State)
	},
}

var gatewayUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Rename a gateway or move its maintenance window",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := parseIDs(args); err != nil {
			return err
		}
		// A literal, not NewPatched...(): constructors seed defaults, and a PATCH
		// must carry only what the operator asked to change.
		body := pidginhost.PatchedGatewayUpdateRequest{
			Name:               givenString(cmd.Flags(), "name"),
			MaintenanceWeekday: givenInt32(cmd.Flags(), "maintenance-weekday"),
			MaintenanceHour:    givenInt32(cmd.Flags(), "maintenance-hour"),
		}
		if err := requireChange(body); err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		g, _, err := c.CloudAPI.CloudGatewaysPartialUpdate(cmd.Context(), args[0]).PatchedGatewayUpdateRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating gateway", err)
		}
		if g == nil {
			return fmt.Errorf("updating gateway: server returned no response")
		}
		if err := checkApplied("updating gateway", body, g, "name"); err != nil {
			return err
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), g,
			"Gateway %d updated (name: %s, maintenance: weekday %d, hour %d)\n",
			g.Id, g.Name, g.MaintenanceWeekday, g.MaintenanceHour)
	},
}

var gatewayDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a gateway and release its networks and IPv4",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := parseIDs(args); err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		op, ok, err := previewed(cmd, fmt.Sprintf("Delete gateway %s?", args[0]),
			func(p *gatewayPreview) (*pidginhost.GatewayOperation, *http.Response, error) {
				req := c.CloudAPI.CloudGatewaysDestroy(cmd.Context(), args[0])
				if p != nil {
					req = req.Revision(p.Revision).Servers(p.serverIDs()).Networks(p.networkIDs())
				}
				return req.Execute()
			})
		if err != nil {
			return cmdutil.APIError("deleting gateway", err)
		}
		if !ok {
			return nil
		}
		return queued(cmd, "Deleting gateway "+args[0], op)
	},
}

var gatewayReenrollCmd = &cobra.Command{
	Use:   "reenroll <id>",
	Short: "Revoke the gateway's agent token and restart it to enroll again",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := parseIDs(args); err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		op, ok, err := previewed(cmd, fmt.Sprintf("Re-enroll gateway %s? It restarts.", args[0]),
			func(p *gatewayPreview) (*pidginhost.GatewayOperation, *http.Response, error) {
				body := *pidginhost.NewGatewayReenrollRequest()
				if p != nil {
					body.Revision = &p.Revision
					body.Servers = p.serverIDs()
					body.Networks = p.networkIDs()
				}
				return c.CloudAPI.CloudGatewaysReenrollCreate(cmd.Context(), args[0]).GatewayReenrollRequest(body).Execute()
			})
		if err != nil {
			return cmdutil.APIError("re-enrolling gateway", err)
		}
		if !ok {
			return nil
		}
		return queued(cmd, "Re-enrolling gateway "+args[0], op)
	},
}

var gatewayStatusCmd = &cobra.Command{
	Use:   "status <id>",
	Short: "Show the gateway's state and its agent's last report",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := parseIDs(args); err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		s, _, err := c.CloudAPI.CloudGatewaysStatusRetrieve(cmd.Context(), args[0]).Execute()
		if err != nil {
			return cmdutil.APIError("getting gateway status", err)
		}
		if s == nil {
			return fmt.Errorf("getting gateway status: server returned no response")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), s, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "State:", s.State)
			if s.StateReason != "" {
				output.PrintRow(tw, "Reason:", s.StateReason)
			}
			output.PrintRow(tw, "Revision:", fmt.Sprintf("%s applied, %d desired, %s attempted",
				output.Pstr(s.AppliedRevision.Get()), s.DesiredRevision, output.Pstr(s.AttemptedRevision.Get())))
			output.PrintRow(tw, "Last Report:", output.Pstr(s.LastStatusAt.Get()))
			output.PrintRow(tw, "Agent:", output.Pstr(s.AgentVersion.Get()))
			output.PrintRow(tw, "Envoy:", output.Pstr(s.EnvoyVersion.Get()))
			output.PrintRow(tw, "Image:", output.Pstr(s.ImageVersion.Get()))
			output.PrintRow(tw, "Ready:", output.Pstr(s.LocalReady.Get()))
			output.PrintRow(tw, "Clock Synced:", output.Pstr(s.ClockSynchronized.Get()))
			output.PrintRow(tw, "Security Updates:", output.Pstr(s.SecurityUpdates.Get()))
			output.PrintRow(tw, "Reboot Required:", output.Pstr(s.RebootRequired.Get()))
			output.PrintRow(tw, "Uptime (s):", output.Pstr(s.UptimeS.Get()))
			if e := s.ApplyError.Get(); e != nil {
				output.PrintRow(tw, "Apply Error:", fmt.Sprintf("%s (rollback: %s)", e.Message, e.Rollback))
			}
			if ct := s.Conntrack.Get(); ct != nil {
				output.PrintRow(tw, "Conntrack:", fmt.Sprintf("%s of %s", output.Pstr(ct.Count.Get()), output.Pstr(ct.Max.Get())))
			}
			if p := s.EgressProbe.Get(); p != nil {
				output.PrintRow(tw, "Egress Probe:", fmt.Sprintf("ok: %s, %s ms", output.Pstr(p.Ok.Get()), output.Pstr(p.LatencyMs.Get())))
			}
			tw.Flush()
			if len(s.Interfaces) > 0 {
				fmt.Fprintln(w)
				tw = output.NewTabWriter(w)
				output.PrintRow(tw, "SLOT", "INTERFACE", "STATE", "ADDRESSES")
				for _, i := range s.Interfaces {
					output.PrintRow(tw, i.Slot, i.Name, i.State, strings.Join(i.Addresses, ", "))
				}
				tw.Flush()
			}
		})
	},
}

var gatewayOperationCmd = &cobra.Command{
	Use:     "operation",
	Aliases: []string{"op"},
	Short:   "Follow a gateway's operations",
	Args:    cobra.NoArgs,
}

var gatewayOperationListCmd = &cobra.Command{
	Use:   "list <gateway-id>",
	Short: "List a gateway's operations, newest first",
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
		ops, err := listAll[pidginhost.GatewayOperation]("gateway operations", func(page int32) (*pidginhost.PaginatedGatewayOperationList, *http.Response, error) {
			return c.CloudAPI.CloudGatewaysOperationsList(cmd.Context(), ids[0]).Page(page).Execute()
		})
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), ops, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "KIND", "STATUS", "STARTED", "COMPLETED", "REASON")
			for _, op := range ops {
				output.PrintRow(tw, op.Id, op.Kind, op.Status, op.StartedAt, output.Pstr(op.CompletedAt.Get()), op.Reason)
			}
			tw.Flush()
		})
	},
}

var gatewayOperationGetCmd = &cobra.Command{
	Use:   "get <gateway-id> <operation-id>",
	Short: "Get one operation",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args[:1])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		op, _, err := c.CloudAPI.CloudGatewaysOperationsRetrieve(cmd.Context(), ids[0], args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting gateway operation", err)
		}
		if op == nil {
			return fmt.Errorf("getting gateway operation: server returned no response")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), op, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", op.Id)
			output.PrintRow(tw, "Kind:", op.Kind)
			output.PrintRow(tw, "Status:", op.Status)
			if op.Reason != "" {
				output.PrintRow(tw, "Reason:", op.Reason)
			}
			output.PrintRow(tw, "Network:", output.Pstr(op.Network.Get()))
			output.PrintRow(tw, "Revision:", output.Pstr(op.Revision.Get()))
			output.PrintRow(tw, "Started:", op.StartedAt)
			output.PrintRow(tw, "Deadline:", output.Pstr(op.DeadlineAt.Get()))
			output.PrintRow(tw, "Completed:", output.Pstr(op.CompletedAt.Get()))
			tw.Flush()
			if len(op.Outcomes) > 0 {
				fmt.Fprintln(w)
				tw = output.NewTabWriter(w)
				output.PrintRow(tw, "SERVER", "STATUS", "REASON")
				for _, server := range op.Servers {
					if o, has := op.Outcomes[fmt.Sprint(server)]; has {
						output.PrintRow(tw, server, o.Status, o.Reason)
					}
				}
				tw.Flush()
			}
		})
	},
}

func init() {
	gatewayCreateCmd.Flags().String("name", "", "Gateway name (required)")
	gatewayCreateCmd.Flags().String("ipv4", "", "An unattached IPv4 address of yours to use (default: allocate a new one)")
	gatewayCreateCmd.Flags().Bool("ipv6", false, "Give the gateway an IPv6 address too")
	gatewayCreateCmd.MarkFlagRequired("name")

	gatewayUpdateCmd.Flags().String("name", "", "New name")
	gatewayUpdateCmd.Flags().Int32("maintenance-weekday", 0, "Maintenance day: 0 (Monday) to 6 (Sunday)")
	gatewayUpdateCmd.Flags().Int32("maintenance-hour", 0, "Maintenance hour: 0 to 23, Bucharest time")

	gatewayOperationCmd.AddCommand(gatewayOperationListCmd)
	gatewayOperationCmd.AddCommand(gatewayOperationGetCmd)

	gatewayCmd.AddCommand(gatewayListCmd)
	gatewayCmd.AddCommand(gatewayGetCmd)
	gatewayCmd.AddCommand(gatewayCreateCmd)
	gatewayCmd.AddCommand(gatewayUpdateCmd)
	gatewayCmd.AddCommand(gatewayDeleteCmd)
	gatewayCmd.AddCommand(gatewayStatusCmd)
	gatewayCmd.AddCommand(gatewayReenrollCmd)
	gatewayCmd.AddCommand(gatewayOperationCmd)
	gatewayCmd.AddCommand(gatewayNetworkCmd)
}
