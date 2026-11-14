package compute

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/output"
)

// asCreate turns the PATCH body the flags built into the create body. The two
// carry the same fields; only create requires some, and decoding names the
// one that is missing.
func asCreate[T any](patch any) (T, error) {
	var body T
	raw, err := json.Marshal(patch)
	if err == nil {
		err = json.Unmarshal(raw, &body)
	}
	return body, err
}

// checkApplied reads a change back out of the response that echoes it: a
// route that answers 200 having applied nothing looks identical to one that
// worked. Only top-level scalars are compared. The API rewrites lists and
// nested objects (it merges port ranges, for one), so those cannot be.
func checkApplied(what string, sent, got any) error {
	want, err := jsonObject(sent)
	if err != nil {
		return fmt.Errorf("%s: reading the change back: %w", what, err)
	}
	have, err := jsonObject(got)
	if err != nil {
		return fmt.Errorf("%s: reading the change back: %w", what, err)
	}
	for _, key := range slices.Sorted(maps.Keys(want)) {
		value := want[key]
		switch v := value.(type) {
		case string:
			value = strings.TrimSpace(v) // the API trims text
		case bool, float64:
		default:
			continue
		}
		if have[key] != value {
			return fmt.Errorf("%s: %s was not applied", what, key)
		}
	}
	return nil
}

func jsonObject(v any) (object map[string]any, err error) {
	raw, err := json.Marshal(v)
	if err == nil {
		err = json.Unmarshal(raw, &object)
	}
	return object, err
}

// requireChange rejects an update that would send an empty PATCH.
func requireChange(body any) error {
	if reflect.ValueOf(body).IsZero() {
		return errors.New("nothing to update: pass at least one option")
	}
	return nil
}

// parsePortRanges reads ports written as "80" or "8000-8100".
func parsePortRanges(flag string, values []string) ([]pidginhost.GatewayPortRangeRequest, error) {
	ranges := make([]pidginhost.GatewayPortRangeRequest, 0, len(values))
	for _, value := range values {
		first, last, isRange := strings.Cut(value, "-")
		if !isRange {
			last = first
		}
		start, errStart := strconv.Atoi(strings.TrimSpace(first))
		end, errEnd := strconv.Atoi(strings.TrimSpace(last))
		if errStart != nil || errEnd != nil || start < 1 || end > 65535 || start > end {
			return nil, fmt.Errorf("--%s: %q is not a port (1-65535) or a range such as 8000-8100", flag, value)
		}
		ranges = append(ranges, *pidginhost.NewGatewayPortRangeRequest(int32(start), int32(end)))
	}
	return ranges, nil
}

func formatPortRange(start, end int32) string {
	if start == end {
		return fmt.Sprint(start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}

// parseSelector reads one side of a firewall rule: "network:<id>", "internet"
// (a destination only) or an IPv4 network.
func parseSelector(flag, value string, allowInternet bool) (network *int32, cidr *string, internet *bool, err error) {
	if id, isNetwork := strings.CutPrefix(value, "network:"); isNetwork {
		n, err := cmdutil.ParseInt32(id)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("--%s: %w", flag, err)
		}
		return &n, nil, nil, nil
	}
	if value == "internet" {
		if !allowInternet {
			return nil, nil, nil, fmt.Errorf("--%s: internet can only be a destination", flag)
		}
		return nil, nil, pidginhost.PtrBool(true), nil
	}
	if strings.TrimSpace(value) == "" {
		return nil, nil, nil, fmt.Errorf("--%s must not be empty", flag)
	}
	return nil, &value, nil, nil
}

func formatSelector(network *int32, cidr *string, internet *bool) string {
	switch {
	case network != nil:
		return fmt.Sprintf("network:%d", *network)
	case internet != nil && *internet:
		return "internet"
	}
	return output.Pstr(cidr)
}

var gatewayFirewallRuleCmd = &cobra.Command{
	Use:     "firewall-rule",
	Aliases: []string{"fw"},
	Short:   "Manage a gateway's firewall rules",
	Long: `Manage a gateway's firewall rules. The first rule that matches wins, in
position order; a new rule goes after the last one.

A source is 'network:<id>' or an IPv4 network such as 10.0.0.0/24. A
destination can also be 'internet'.`,
	Args: cobra.NoArgs,
}

func printGatewayFirewallRules(w io.Writer, rules ...pidginhost.GatewayFirewallRule) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "ID", "POS", "ACTION", "SOURCE", "DESTINATION", "PROTO", "PORTS", "ENABLED", "PENDING", "COMMENT")
	for _, r := range rules {
		ports := make([]string, len(r.Ports))
		for i, p := range r.Ports {
			ports[i] = formatPortRange(p.Start, p.End)
		}
		output.PrintRow(tw, r.Id, r.Position, r.Action,
			formatSelector(r.Source.Network, r.Source.Cidr, nil),
			formatSelector(r.Destination.Network, r.Destination.Cidr, r.Destination.Internet),
			r.Protocol, strings.Join(ports, ","), r.Enabled, r.Pending, r.Comment)
	}
	tw.Flush()
}

var gatewayFirewallRuleListCmd = &cobra.Command{
	Use:   "list <gateway-id>",
	Short: "List firewall rules in evaluation order",
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
		rules, err := listAll[pidginhost.GatewayFirewallRule]("gateway firewall rules", func(page int32) (*pidginhost.PaginatedGatewayFirewallRuleList, *http.Response, error) {
			return c.CloudAPI.CloudGatewaysFirewallRulesList(cmd.Context(), ids[0]).Page(page).Execute()
		})
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), rules, func(w io.Writer) {
			printGatewayFirewallRules(w, rules...)
		})
	},
}

var gatewayFirewallRuleGetCmd = &cobra.Command{
	Use:   "get <gateway-id> <rule-id>",
	Short: "Get one firewall rule",
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
		rule, _, err := c.CloudAPI.CloudGatewaysFirewallRulesRetrieve(cmd.Context(), ids[0], ids[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting firewall rule", err)
		}
		if rule == nil {
			return fmt.Errorf("getting firewall rule: server returned no response")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), rule, func(w io.Writer) {
			printGatewayFirewallRules(w, *rule)
		})
	},
}

func addGatewayFirewallRuleFlags(cmd *cobra.Command) {
	cmd.Flags().String("action", "", "ACCEPT, DROP or REJECT")
	cmd.Flags().String("source", "", "Where the traffic comes from: network:<id> or an IPv4 network")
	cmd.Flags().String("destination", "", "Where it goes: network:<id>, an IPv4 network, or internet")
	cmd.Flags().String("protocol", "", "tcp, udp, icmp or any")
	cmd.Flags().StringSlice("ports", nil, "Destination ports, such as 80,8000-8100 (tcp and udp only)")
	cmd.Flags().Bool("enabled", true, "Whether the rule is evaluated")
	cmd.Flags().String("comment", "", "A note for yourself")
}

// gatewayFirewallRuleBody carries only the flags that were given.
func gatewayFirewallRuleBody(cmd *cobra.Command) (body pidginhost.PatchedGatewayFirewallRuleWriteRequest, err error) {
	flags := cmd.Flags()
	if flags.Changed("action") {
		action, _ := flags.GetString("action")
		if err := oneOf("action", action, "ACCEPT", "DROP", "REJECT"); err != nil {
			return body, err
		}
		body.Action = pidginhost.FwPolicyOutEnum(action).Ptr()
	}
	if flags.Changed("source") {
		value, _ := flags.GetString("source")
		var source pidginhost.GatewayFirewallSourceRequest
		if source.Network, source.Cidr, _, err = parseSelector("source", value, false); err != nil {
			return body, err
		}
		body.Source = &source
	}
	if flags.Changed("destination") {
		value, _ := flags.GetString("destination")
		var destination pidginhost.GatewayFirewallDestinationRequest
		if destination.Network, destination.Cidr, destination.Internet, err = parseSelector("destination", value, true); err != nil {
			return body, err
		}
		body.Destination = &destination
	}
	if flags.Changed("protocol") {
		protocol, _ := flags.GetString("protocol")
		if err := oneOf("protocol", protocol, "tcp", "udp", "icmp", "any"); err != nil {
			return body, err
		}
		body.Protocol = pidginhost.GatewayFirewallProtocolEnum(protocol).Ptr()
	}
	if flags.Changed("ports") {
		ports, _ := flags.GetStringSlice("ports")
		if body.Ports, err = parsePortRanges("ports", ports); err != nil {
			return body, err
		}
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

var gatewayFirewallRuleCreateCmd = &cobra.Command{
	Use:   "create <gateway-id>",
	Short: "Add a firewall rule after the last one",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		patch, err := gatewayFirewallRuleBody(cmd)
		if err != nil {
			return err
		}
		body, err := asCreate[pidginhost.GatewayFirewallRuleWriteRequest](patch)
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		rule, _, err := c.CloudAPI.CloudGatewaysFirewallRulesCreate(cmd.Context(), ids[0]).GatewayFirewallRuleWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating firewall rule", err)
		}
		if rule == nil {
			return fmt.Errorf("creating firewall rule: server returned no response")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), rule,
			"Firewall rule created (ID: %d, position: %d, pending: %v)\n", rule.Id, rule.Position, rule.Pending)
	},
}

var gatewayFirewallRuleUpdateCmd = &cobra.Command{
	Use:   "update <gateway-id> <rule-id>",
	Short: "Change a firewall rule; its position stays",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		body, err := gatewayFirewallRuleBody(cmd)
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
		rule, _, err := c.CloudAPI.CloudGatewaysFirewallRulesPartialUpdate(cmd.Context(), ids[0], ids[1]).PatchedGatewayFirewallRuleWriteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating firewall rule", err)
		}
		if rule == nil {
			return fmt.Errorf("updating firewall rule: server returned no response")
		}
		if err := checkApplied("updating firewall rule", body, rule); err != nil {
			return err
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), rule,
			"Firewall rule %d updated (pending: %v)\n", rule.Id, rule.Pending)
	},
}

var gatewayFirewallRuleDeleteCmd = &cobra.Command{
	Use:     "delete <gateway-id> <rule-id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a firewall rule",
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
		op, ok, err := previewed(cmd, fmt.Sprintf("Delete firewall rule %d of gateway %d?", ids[1], ids[0]),
			func(p *gatewayPreview) (*pidginhost.GatewayOperation, *http.Response, error) {
				req := c.CloudAPI.CloudGatewaysFirewallRulesDestroy(cmd.Context(), ids[0], ids[1])
				if p != nil {
					req = req.Revision(p.Revision)
				}
				return req.Execute()
			})
		if err != nil {
			return cmdutil.APIError("deleting firewall rule", err)
		}
		if !ok {
			return nil
		}
		return queued(cmd, fmt.Sprintf("Deleting firewall rule %d", ids[1]), op)
	},
}

var gatewayFirewallRuleMoveCmd = &cobra.Command{
	Use:   "move <gateway-id> <rule-id>",
	Short: "Move a firewall rule to a position; the others keep their order",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}
		position, _ := cmd.Flags().GetInt32("position")
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewGatewayFirewallMoveRequest(position)
		rule, ok, err := previewed(cmd, fmt.Sprintf("Move firewall rule %d of gateway %d to position %d?", ids[1], ids[0], position),
			func(p *gatewayPreview) (*pidginhost.GatewayFirewallRule, *http.Response, error) {
				req := c.CloudAPI.CloudGatewaysFirewallRulesMoveCreate(cmd.Context(), ids[0], ids[1]).GatewayFirewallMoveRequest(body)
				if p != nil {
					req = req.Revision(p.Revision)
				}
				return req.Execute()
			})
		if err != nil {
			return cmdutil.APIError("moving firewall rule", err)
		}
		if !ok {
			return nil
		}
		if rule == nil {
			return fmt.Errorf("moving firewall rule: server returned no response")
		}
		if err := checkApplied("moving firewall rule", body, rule); err != nil {
			return err
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), rule,
			"Firewall rule %d moved to position %d (pending: %v)\n", rule.Id, rule.Position, rule.Pending)
	},
}

func init() {
	addGatewayFirewallRuleFlags(gatewayFirewallRuleCreateCmd)
	for _, name := range []string{"action", "source", "destination", "protocol"} {
		gatewayFirewallRuleCreateCmd.MarkFlagRequired(name)
	}
	addGatewayFirewallRuleFlags(gatewayFirewallRuleUpdateCmd)

	gatewayFirewallRuleMoveCmd.Flags().Int32("position", 0, "Position to move the rule to, from 0 (required)")
	gatewayFirewallRuleMoveCmd.MarkFlagRequired("position")

	gatewayFirewallRuleCmd.AddCommand(gatewayFirewallRuleListCmd)
	gatewayFirewallRuleCmd.AddCommand(gatewayFirewallRuleGetCmd)
	gatewayFirewallRuleCmd.AddCommand(gatewayFirewallRuleCreateCmd)
	gatewayFirewallRuleCmd.AddCommand(gatewayFirewallRuleUpdateCmd)
	gatewayFirewallRuleCmd.AddCommand(gatewayFirewallRuleDeleteCmd)
	gatewayFirewallRuleCmd.AddCommand(gatewayFirewallRuleMoveCmd)
	gatewayCmd.AddCommand(gatewayFirewallRuleCmd)
}
