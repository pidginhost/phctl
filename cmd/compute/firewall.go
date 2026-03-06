package compute

import (
	"context"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var firewallCmd = &cobra.Command{
	Use:     "firewall",
	Aliases: []string{"fw"},
	Short:   "Manage firewall rule sets",
}

var firewallListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all firewall rule sets",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudFirewallRulesSetList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing firewalls: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, resp, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "STATUS", "RULES")
			for _, f := range resp {
				output.PrintRow(tw, f.Id, f.Name, f.Status, len(f.Rules))
			}
			tw.Flush()
		})
	},
}

var firewallGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get firewall rule set details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		fw, _, err := c.CloudAPI.CloudFirewallRulesSetRetrieve(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("getting firewall: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, fw, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", fw.Id)
			output.PrintRow(tw, "Name:", fw.Name)
			output.PrintRow(tw, "Status:", fw.Status)
			output.PrintRow(tw, "Read Only:", fw.ReadOnly)
			tw.Flush()
			if len(fw.Rules) > 0 {
				fmt.Fprintln(w)
				rw := output.NewTabWriter(w)
				output.PrintRow(rw, "RULE ID", "DIR", "ACTION", "PROTO", "SPORT", "DPORT", "SOURCE", "DEST", "ENABLED")
				for _, r := range fw.Rules {
					output.PrintRow(rw, r.Id, r.Direction, r.Action, pstr(r.Protocol), pstr(r.Sport), pstr(r.Dport), pstr(r.Source), pstr(r.Destination), pstr(r.Enabled))
				}
				rw.Flush()
			}
		})
	},
}

var firewallCreateName string

var firewallCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a firewall rule set",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewFirewallRulesSet(0, firewallCreateName, "", []pidginhost.FirewallRule{}, false)
		resp, _, err := c.CloudAPI.CloudFirewallRulesSetCreate(context.Background()).FirewallRulesSet(body).Execute()
		if err != nil {
			return fmt.Errorf("creating firewall: %w", err)
		}
		fmt.Printf("Firewall rule set created (ID: %d, Name: %s)\n", resp.Id, resp.Name)
		return nil
	},
}

var firewallDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a firewall rule set",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete firewall rule set %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudFirewallRulesSetDestroy(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("deleting firewall: %w", err)
		}
		fmt.Printf("Firewall rule set %d deleted.\n", id)
		return nil
	},
}

// --- Rules subcommands ---

var ruleCmd = &cobra.Command{
	Use:   "rule",
	Short: "Manage individual firewall rules",
}

var ruleListCmd = &cobra.Command{
	Use:   "list <ruleset-id>",
	Short: "List rules in a firewall rule set",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudFirewallRulesSetRulesList(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("listing rules: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, resp, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "DIR", "ACTION", "PROTO", "SPORT", "DPORT", "SOURCE", "DEST", "ENABLED")
			for _, r := range resp {
				output.PrintRow(tw, r.Id, r.Direction, r.Action, pstr(r.Protocol), pstr(r.Sport), pstr(r.Dport), pstr(r.Source), pstr(r.Destination), pstr(r.Enabled))
			}
			tw.Flush()
		})
	},
}

var (
	ruleDirection   string
	ruleAction      string
	ruleProtocol    string
	ruleSource      string
	ruleDport       string
	ruleSport       string
	ruleDestination string
)

var ruleCreateCmd = &cobra.Command{
	Use:   "create <ruleset-id>",
	Short: "Add a rule to a firewall rule set",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewFirewallRule(
			0,
			pidginhost.FirewallRuleDirectionEnum(ruleDirection),
			pidginhost.FwPolicyOutEnum(ruleAction),
			false, "",
		)
		if ruleProtocol != "" {
			body.Protocol = pidginhost.PtrString(ruleProtocol)
		}
		if ruleSource != "" {
			body.Source = pidginhost.PtrString(ruleSource)
		}
		if ruleDport != "" {
			body.Dport = pidginhost.PtrString(ruleDport)
		}
		if ruleSport != "" {
			body.Sport = pidginhost.PtrString(ruleSport)
		}
		if ruleDestination != "" {
			body.Destination = pidginhost.PtrString(ruleDestination)
		}

		resp, _, err := c.CloudAPI.CloudFirewallRulesSetRulesCreate(context.Background(), args[0]).FirewallRule(body).Execute()
		if err != nil {
			return fmt.Errorf("creating rule: %w", err)
		}
		fmt.Printf("Rule created (ID: %d, Direction: %s, Action: %s)\n", resp.Id, resp.Direction, resp.Action)
		return nil
	},
}

var ruleDeleteCmd = &cobra.Command{
	Use:     "delete <ruleset-id> <rule-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a rule from a firewall rule set",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete rule %s from ruleset %s?", args[1], args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudFirewallRulesSetRulesDestroy(context.Background(), args[1], args[0]).Execute()
		if err != nil {
			return fmt.Errorf("deleting rule: %w", err)
		}
		fmt.Printf("Rule %s deleted from ruleset %s.\n", args[1], args[0])
		return nil
	},
}

func init() {
	firewallCreateCmd.Flags().StringVar(&firewallCreateName, "name", "", "Rule set name (required)")
	firewallCreateCmd.MarkFlagRequired("name")

	ruleCreateCmd.Flags().StringVar(&ruleDirection, "direction", "", "Direction: in or out (required)")
	ruleCreateCmd.Flags().StringVar(&ruleAction, "action", "", "Action: ACCEPT or DROP (required)")
	ruleCreateCmd.Flags().StringVar(&ruleProtocol, "protocol", "", "Protocol (tcp, udp, icmp, etc.)")
	ruleCreateCmd.Flags().StringVar(&ruleSource, "source", "", "Source IP/range/list")
	ruleCreateCmd.Flags().StringVar(&ruleDport, "dport", "", "Destination port(s)")
	ruleCreateCmd.Flags().StringVar(&ruleSport, "sport", "", "Source port(s)")
	ruleCreateCmd.Flags().StringVar(&ruleDestination, "destination", "", "Destination IP/range/list")
	ruleCreateCmd.MarkFlagRequired("direction")
	ruleCreateCmd.MarkFlagRequired("action")

	ruleCmd.AddCommand(ruleListCmd)
	ruleCmd.AddCommand(ruleCreateCmd)
	ruleCmd.AddCommand(ruleDeleteCmd)

	firewallCmd.AddCommand(firewallListCmd)
	firewallCmd.AddCommand(firewallGetCmd)
	firewallCmd.AddCommand(firewallCreateCmd)
	firewallCmd.AddCommand(firewallDeleteCmd)
	firewallCmd.AddCommand(ruleCmd)
}
