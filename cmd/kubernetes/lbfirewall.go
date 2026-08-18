package kubernetes

import (
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

// Load balancer firewall rules guard the public IP of a cluster's LB VM.

var lbFirewallCmd = &cobra.Command{
	Use:     "lb-firewall",
	Aliases: []string{"lbfw"},
	Short:   "Manage load balancer firewall rules",
	Args:    cobra.NoArgs,
}

// lbRuleFlags carries the writable fields of a rule. create and update each own
// an instance so that "was this flag given?" stays per-command.
type lbRuleFlags struct {
	owner *cobra.Command

	direction   string
	action      string
	protocol    string
	source      string
	sport       string
	destination string
	dport       string
	comment     string
	enabled     bool
	position    int32
}

// fieldFlags are the flags that carry a rule field, in the order they are
// reported. Anything outside this list is not part of the request body.
var lbRuleFieldFlags = []string{
	"direction", "action", "protocol", "source", "sport",
	"destination", "dport", "comment", "enabled", "position",
}

func (f *lbRuleFlags) register(cmd *cobra.Command) {
	f.owner = cmd
	cmd.Flags().StringVar(&f.direction, "direction", "in", "Direction: in or out")
	cmd.Flags().StringVar(&f.action, "action", "ACCEPT", "Action: ACCEPT, DROP or REJECT")
	cmd.Flags().StringVar(&f.protocol, "protocol", "", "Protocol (tcp, udp, icmp, etc.)")
	cmd.Flags().StringVar(&f.source, "source", "", "Source IP or CIDR")
	cmd.Flags().StringVar(&f.sport, "sport", "", "Source port or range (e.g. 1024-65535)")
	cmd.Flags().StringVar(&f.destination, "destination", "", "Destination IP or CIDR")
	cmd.Flags().StringVar(&f.dport, "dport", "", "Destination port or range (e.g. 80, 8000-9000)")
	cmd.Flags().StringVar(&f.comment, "comment", "", "Free-form comment")
	cmd.Flags().BoolVar(&f.enabled, "enabled", true, "Whether the rule is active")
	cmd.Flags().Int32Var(&f.position, "position", 100, "Rule order (lower = higher priority)")
}

// isSet reports whether the caller actually gave the flag. Defaults are left
// out of the request so the API keeps applying its own.
func (f *lbRuleFlags) isSet(name string) bool {
	return f.owner != nil && f.owner.Flags().Changed(name)
}

func (f *lbRuleFlags) anySet() bool {
	for _, name := range lbRuleFieldFlags {
		if f.isSet(name) {
			return true
		}
	}
	return false
}

// direction and action are enums server-side. Validate here so a typo fails
// before it reaches a live load balancer.
func (f *lbRuleFlags) parsedDirection() (*pidginhost.LBFirewallRuleDirectionEnum, error) {
	v, err := pidginhost.NewLBFirewallRuleDirectionEnumFromValue(f.direction)
	if err != nil {
		return nil, fmt.Errorf("invalid --direction %q: must be in or out", f.direction)
	}
	return v, nil
}

func (f *lbRuleFlags) parsedAction() (*pidginhost.LBFirewallRuleActionEnum, error) {
	v, err := pidginhost.NewLBFirewallRuleActionEnumFromValue(f.action)
	if err != nil {
		return nil, fmt.Errorf("invalid --action %q: must be ACCEPT, DROP or REJECT", f.action)
	}
	return v, nil
}

var (
	lbFirewallCreateFlags lbRuleFlags
	lbFirewallUpdateFlags lbRuleFlags
)

func printLBRule(w io.Writer, r *pidginhost.LBFirewallRule) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "ID:", r.Id)
	output.PrintRow(tw, "Direction:", output.Pstr(r.Direction))
	output.PrintRow(tw, "Action:", output.Pstr(r.Action))
	output.PrintRow(tw, "Protocol:", output.Pstr(r.Protocol))
	output.PrintRow(tw, "Source:", output.Pstr(r.Source))
	output.PrintRow(tw, "Source port:", output.Pstr(r.Sport))
	output.PrintRow(tw, "Destination:", output.Pstr(r.Destination))
	output.PrintRow(tw, "Destination port:", output.Pstr(r.Dport))
	output.PrintRow(tw, "Comment:", output.Pstr(r.Comment))
	output.PrintRow(tw, "Enabled:", output.Pstr(r.Enabled))
	output.PrintRow(tw, "Position:", output.Pstr(r.Position))
	output.PrintRow(tw, "Created:", r.Created)
	output.PrintRow(tw, "Updated:", r.Updated)
	tw.Flush()
}

var lbFirewallListCmd = &cobra.Command{
	Use:   "list <cluster-id>",
	Short: "List load balancer firewall rules",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		rules, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.LBFirewallRule, bool, error) {
			resp, _, err := c.KubernetesAPI.KubernetesClustersLbFirewallList(cmd.Context(), clusterID).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			if resp == nil {
				return nil, false, fmt.Errorf("server returned no rule list")
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing LB firewall rules", err)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), rules, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "POS", "DIRECTION", "ACTION", "PROTO", "SOURCE", "SPORT", "DESTINATION", "DPORT", "ENABLED", "COMMENT")
			for _, r := range rules {
				output.PrintRow(tw, r.Id, output.Pstr(r.Position), output.Pstr(r.Direction), output.Pstr(r.Action),
					output.Pstr(r.Protocol), output.Pstr(r.Source), output.Pstr(r.Sport),
					output.Pstr(r.Destination), output.Pstr(r.Dport), output.Pstr(r.Enabled), output.Pstr(r.Comment))
			}
			tw.Flush()
		})
	},
}

var lbFirewallGetCmd = &cobra.Command{
	Use:   "get <cluster-id> <rule-id>",
	Short: "Get a load balancer firewall rule",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		ruleID, err := cmdutil.ParseInt32(args[1])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		rule, _, err := c.KubernetesAPI.KubernetesClustersLbFirewallRetrieve(cmd.Context(), clusterID, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting LB firewall rule", err)
		}
		if rule == nil {
			return fmt.Errorf("getting LB firewall rule %d: server returned no rule", ruleID)
		}
		if rule.Id != ruleID {
			return fmt.Errorf("getting LB firewall rule %d: server returned rule %d", ruleID, rule.Id)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), rule, func(w io.Writer) {
			printLBRule(w, rule)
		})
	},
}

var lbFirewallCreateCmd = &cobra.Command{
	Use:   "create <cluster-id>",
	Short: "Create a load balancer firewall rule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		f := &lbFirewallCreateFlags
		direction, err := f.parsedDirection()
		if err != nil {
			return err
		}
		action, err := f.parsedAction()
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		// id, created and updated are read-only server-side; the schema reuses
		// the response component for the request body, so they must be present.
		body := *pidginhost.NewLBFirewallRule(0, "", "")
		body.Direction = direction
		body.Action = action
		f.applyOptional(&body)

		rule, _, err := c.KubernetesAPI.KubernetesClustersLbFirewallCreate(cmd.Context(), clusterID).LBFirewallRule(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating LB firewall rule", err)
		}
		if rule == nil {
			return fmt.Errorf("creating LB firewall rule: server returned no rule")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), rule,
			"LB firewall rule created (ID: %d, %s %s).\n", rule.Id, output.Pstr(rule.Direction), output.Pstr(rule.Action))
	},
}

// applyOptional copies the optional fields the caller actually gave, leaving
// the rest to the server's own defaults.
func (f *lbRuleFlags) applyOptional(body *pidginhost.LBFirewallRule) {
	if f.isSet("protocol") {
		body.Protocol = pidginhost.PtrString(f.protocol)
	}
	if f.isSet("source") {
		body.Source = pidginhost.PtrString(f.source)
	}
	if f.isSet("sport") {
		body.Sport = pidginhost.PtrString(f.sport)
	}
	if f.isSet("destination") {
		body.Destination = pidginhost.PtrString(f.destination)
	}
	if f.isSet("dport") {
		body.Dport = pidginhost.PtrString(f.dport)
	}
	if f.isSet("comment") {
		body.Comment = pidginhost.PtrString(f.comment)
	}
	if f.isSet("enabled") {
		body.Enabled = pidginhost.PtrBool(f.enabled)
	}
	if f.isSet("position") {
		body.Position = pidginhost.PtrInt32(f.position)
	}
}

var lbFirewallUpdateCmd = &cobra.Command{
	Use:   "update <cluster-id> <rule-id>",
	Short: "Update a load balancer firewall rule",
	Long: "Update a load balancer firewall rule.\n\n" +
		"Only the fields you pass are sent. Applying an update withdraws the rule\n" +
		"from the load balancer and re-adds it, so the traffic it governs is\n" +
		"briefly unfiltered.",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		ruleID, err := cmdutil.ParseInt32(args[1])
		if err != nil {
			return err
		}
		f := &lbFirewallUpdateFlags
		// A PATCH with an empty body answers 200 having changed nothing.
		if !f.anySet() {
			return fmt.Errorf("nothing to update: pass at least one of --%s",
				joinFlags(lbRuleFieldFlags))
		}

		body := *pidginhost.NewPatchedLBFirewallRule()
		if f.isSet("direction") {
			direction, err := f.parsedDirection()
			if err != nil {
				return err
			}
			body.Direction = direction
		}
		if f.isSet("action") {
			action, err := f.parsedAction()
			if err != nil {
				return err
			}
			body.Action = action
		}
		f.applyOptionalPatched(&body)

		c, err := newClient()
		if err != nil {
			return err
		}
		rule, _, err := c.KubernetesAPI.KubernetesClustersLbFirewallPartialUpdate(cmd.Context(), clusterID, args[1]).
			PatchedLBFirewallRule(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating LB firewall rule", err)
		}
		if rule == nil {
			return fmt.Errorf("updating LB firewall rule %d: server returned no rule", ruleID)
		}
		if rule.Id != ruleID {
			return fmt.Errorf("updating LB firewall rule %d: server returned rule %d", ruleID, rule.Id)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), rule,
			"LB firewall rule %d updated.\n", rule.Id)
	},
}

func (f *lbRuleFlags) applyOptionalPatched(body *pidginhost.PatchedLBFirewallRule) {
	if f.isSet("protocol") {
		body.Protocol = pidginhost.PtrString(f.protocol)
	}
	if f.isSet("source") {
		body.Source = pidginhost.PtrString(f.source)
	}
	if f.isSet("sport") {
		body.Sport = pidginhost.PtrString(f.sport)
	}
	if f.isSet("destination") {
		body.Destination = pidginhost.PtrString(f.destination)
	}
	if f.isSet("dport") {
		body.Dport = pidginhost.PtrString(f.dport)
	}
	if f.isSet("comment") {
		body.Comment = pidginhost.PtrString(f.comment)
	}
	if f.isSet("enabled") {
		body.Enabled = pidginhost.PtrBool(f.enabled)
	}
	if f.isSet("position") {
		body.Position = pidginhost.PtrInt32(f.position)
	}
}

var lbFirewallDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <rule-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a load balancer firewall rule",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if _, err := cmdutil.ParseInt32(args[1]); err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Delete LB firewall rule %s from cluster %d? Traffic it blocked will be allowed by the remaining rules.", args[1], clusterID)) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersLbFirewallDestroy(cmd.Context(), clusterID, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("deleting LB firewall rule", err)
		}
		cmd.Printf("LB firewall rule %s deleted.\n", args[1])
		return nil
	},
}

// joinFlags renders a flag-name list for an error message.
func joinFlags(names []string) string {
	out := ""
	for i, n := range names {
		switch {
		case i == 0:
			out = n
		case i == len(names)-1:
			out += " or --" + n
		default:
			out += ", --" + n
		}
	}
	return out
}

func init() {
	lbFirewallCreateFlags.register(lbFirewallCreateCmd)
	lbFirewallUpdateFlags.register(lbFirewallUpdateCmd)

	lbFirewallCmd.AddCommand(lbFirewallListCmd)
	lbFirewallCmd.AddCommand(lbFirewallGetCmd)
	lbFirewallCmd.AddCommand(lbFirewallCreateCmd)
	lbFirewallCmd.AddCommand(lbFirewallUpdateCmd)
	lbFirewallCmd.AddCommand(lbFirewallDeleteCmd)

	Cmd.AddCommand(lbFirewallCmd)
}
