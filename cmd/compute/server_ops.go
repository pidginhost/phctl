package compute

import (
	"fmt"
	"io"
	"strings"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

// --- rescue ---

var serverRescueCmd = &cobra.Command{
	Use:   "rescue",
	Short: "Boot a server into rescue media, or back to its disk",
	Args:  cobra.NoArgs,
}

var serverRescueEnterCmd = &cobra.Command{
	Use:   "enter <id>",
	Short: "Reboot a server into rescue media",
	Long: "Reboot a server into rescue media.\n\n" +
		"Without --iso the server boots the default rescue image. " +
		"List the alternatives with `phctl compute server boot-isos`; some of them " +
		"are installers that wipe the disk.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		iso, err := cmd.Flags().GetString("iso")
		if err != nil {
			return err
		}
		isoSet := cmd.Flags().Changed("iso")
		if isoSet && iso == "" {
			return fmt.Errorf("--iso requires a non-empty slug")
		}
		target := "the default rescue image"
		if isoSet {
			target = fmt.Sprintf("%q", iso)
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Reboot server %d into %s? It stops serving until it leaves rescue.", id, target)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := pidginhost.NewIsoBootRequest()
		// Omit the field entirely so the server picks its default; sending an
		// empty slug is a different request.
		if isoSet {
			body.SetIso(iso)
		}
		resp, _, err := c.CloudAPI.CloudServersRescueEnterCreate(cmd.Context(), id).IsoBootRequest(*body).Execute()
		if err != nil {
			return cmdutil.APIError("entering rescue mode", err)
		}
		if resp == nil {
			return fmt.Errorf("entering rescue mode on server %d: server returned no result", id)
		}
		if !resp.Queued {
			return fmt.Errorf("entering rescue mode on server %d: server did not queue the reboot", id)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"Server %d queued to boot into %s.\n", id, target)
	},
}

var serverRescueExitCmd = &cobra.Command{
	Use:   "exit <id>",
	Short: "Reboot a server out of rescue media, back to its disk",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Reboot server %d out of rescue and back to its disk?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudServersRescueExitCreate(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("exiting rescue mode", err)
		}
		if resp == nil {
			return fmt.Errorf("exiting rescue mode on server %d: server returned no result", id)
		}
		if !resp.Queued {
			return fmt.Errorf("exiting rescue mode on server %d: server did not queue the reboot", id)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"Server %d queued to boot from its disk.\n", id)
	},
}

// --- boot ISOs ---

// The catalog is per-server: compatibility depends on the server's own disk
// and RAM, so it is scoped to an id rather than listed globally.
var serverBootISOsCmd = &cobra.Command{
	Use:     "boot-isos <id>",
	Aliases: []string{"boot-iso", "isos"},
	Short:   "List the rescue and installer images a server can boot",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		// The route answers a bare array, not a paginated envelope.
		isos, _, err := c.CloudAPI.CloudServersBootIsosList(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("listing boot ISOs", err)
		}
		if isos == nil {
			return fmt.Errorf("listing boot ISOs for server %d: server returned no boot ISO list", id)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), isos, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "SLUG", "NAME", "CATEGORY", "MIN DISK GB", "MIN RAM GB", "WIPES DISK", "COMPATIBLE")
			for _, iso := range isos {
				category := ""
				if iso.Category != nil {
					category = string(*iso.Category)
				}
				// MinRam is a decimal the API sends as a string. Never a float verb.
				output.PrintRow(tw, iso.Slug, iso.Name, category,
					output.Pstr(iso.MinDiskSize), output.Pstr(iso.MinRam),
					output.Pstr(iso.WipesDisk), iso.Compatible)
			}
			tw.Flush()
		})
	},
}

// --- usage ---

var serverUsageCmd = &cobra.Command{
	Use:   "usage <id>",
	Short: "Show a server's current status, uptime, CPU and memory",
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
		resp, _, err := c.CloudAPI.CloudServersUsageRetrieve(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("getting server usage", err)
		}
		if resp == nil {
			return fmt.Errorf("getting server usage: server returned no result")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "Status:", resp.Status)
			output.PrintRow(tw, "Uptime:", resp.UptimeText)
			output.PrintRow(tw, "Uptime (s):", resp.Uptime)
			// cpu and memory are free-form maps; print them sorted so the same
			// response renders identically between runs.
			for _, row := range sortedMapRows("CPU", resp.Cpu) {
				output.PrintRow(tw, row[0], row[1])
			}
			for _, row := range sortedMapRows("Memory", resp.Memory) {
				output.PrintRow(tw, row[0], row[1])
			}
			tw.Flush()
		})
	},
}

// sortedMapRows renders a free-form metrics map as stable "Prefix key:" rows.
func sortedMapRows(prefix string, m map[string]interface{}) [][2]string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	rows := make([][2]string, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, [2]string{fmt.Sprintf("%s %s:", prefix, k), fmt.Sprintf("%v", m[k])})
	}
	return rows
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && strings.Compare(s[j-1], s[j]) > 0; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// --- activity ---

var serverActivityCmd = &cobra.Command{
	Use:     "activity <id>",
	Aliases: []string{"log", "activity-log"},
	Short:   "Show a server's activity log",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudServersActivityRetrieve(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("getting server activity", err)
		}
		if resp == nil {
			return fmt.Errorf("getting server activity: server returned no result")
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "DATE", "MESSAGE")
			for _, e := range resp.Logs {
				output.PrintRow(tw, e.Date, e.Message)
			}
			tw.Flush()
		})
	},
}

// --- retry provision ---

var serverRetryProvisionCmd = &cobra.Command{
	Use:   "retry-provision <id>",
	Short: "Retry provisioning a server that failed to build",
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
		resp, _, err := c.CloudAPI.CloudServersRetryProvisionCreate(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("retrying provision", err)
		}
		if resp == nil {
			return fmt.Errorf("retrying provision on server %d: server returned no result", id)
		}
		if !resp.Retry {
			return fmt.Errorf("retrying provision on server %d: server did not accept the retry", id)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"Server %d queued for another provisioning attempt.\n", id)
	},
}

// --- public interface ---

var serverPublicInterfaceCmd = &cobra.Command{
	Use:     "public-interface",
	Aliases: []string{"public-iface"},
	Short:   "Inspect, configure or remove a server's public interface",
	Args:    cobra.NoArgs,
}

func printPublicInterface(cmd *cobra.Command, pi *pidginhost.PublicInterface) error {
	if pi == nil {
		return fmt.Errorf("server returned no public interface")
	}
	return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), pi, func(w io.Writer) {
		tw := output.NewTabWriter(w)
		output.PrintRow(tw, "Interface:", pi.Interface)
		output.PrintRow(tw, "IPv4:", pi.Ipv4)
		output.PrintRow(tw, "IPv6:", pi.Ipv6)
		output.PrintRow(tw, "Firewall set:", output.Pstr(pi.FwRulesSet.Get()))
		if pi.FwPolicyIn != nil {
			output.PrintRow(tw, "Policy in:", string(*pi.FwPolicyIn))
		}
		if pi.FwPolicyOut != nil {
			output.PrintRow(tw, "Policy out:", string(*pi.FwPolicyOut))
		}
		tw.Flush()
	})
}

var serverPublicInterfaceGetCmd = &cobra.Command{
	Use:   "get <server-id>",
	Short: "Show a server's public interface",
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
		pi, _, err := c.CloudAPI.CloudServersPublicInterfaceRetrieve(cmd.Context(), id).Execute()
		if err != nil {
			return cmdutil.APIError("getting public interface", err)
		}
		return printPublicInterface(cmd, pi)
	},
}

var (
	serverPublicInterfaceFirewall  string
	serverPublicInterfacePolicyIn  string
	serverPublicInterfacePolicyOut string
)

// parseFwPolicy keeps an unknown policy from reaching the API as a silent no-op.
func parseFwPolicy(flag, value string) (pidginhost.FwPolicyOutEnum, error) {
	for _, allowed := range pidginhost.AllowedFwPolicyOutEnumEnumValues {
		if value == string(allowed) {
			return allowed, nil
		}
	}
	return "", fmt.Errorf("--%s must be one of ACCEPT, DROP, REJECT (got %q)", flag, value)
}

var serverPublicInterfaceSetCmd = &cobra.Command{
	Use:   "set <server-id>",
	Short: "Set the firewall rule set or policies on a server's public interface",
	Long: "Set the firewall rule set or policies on a server's public interface.\n\n" +
		"The addresses and interface name are assigned by the platform and cannot " +
		"be changed here; only the firewall settings are writable.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if serverPublicInterfaceFirewall == "" &&
			serverPublicInterfacePolicyIn == "" && serverPublicInterfacePolicyOut == "" {
			return fmt.Errorf("pass at least one of --firewall, --policy-in or --policy-out")
		}
		// Only the firewall settings are writable; the request model carries
		// nothing else.
		body := pidginhost.NewPublicInterfaceRequest()
		if serverPublicInterfaceFirewall != "" {
			body.SetFwRulesSet(serverPublicInterfaceFirewall)
		}
		if serverPublicInterfacePolicyIn != "" {
			policy, err := parseFwPolicy("policy-in", serverPublicInterfacePolicyIn)
			if err != nil {
				return err
			}
			body.SetFwPolicyIn(policy)
		}
		if serverPublicInterfacePolicyOut != "" {
			policy, err := parseFwPolicy("policy-out", serverPublicInterfacePolicyOut)
			if err != nil {
				return err
			}
			body.SetFwPolicyOut(policy)
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		pi, _, err := c.CloudAPI.CloudServersPublicInterfaceCreate(cmd.Context(), id).PublicInterfaceRequest(*body).Execute()
		if err != nil {
			return cmdutil.APIError("setting public interface", err)
		}
		return printPublicInterface(cmd, pi)
	},
}

var serverPublicInterfaceDeleteCmd = &cobra.Command{
	Use:     "delete <server-id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Remove a server's public interface",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Remove server %d's public interface? It loses public connectivity.", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		if _, err := c.CloudAPI.CloudServersPublicInterfaceDestroy(cmd.Context(), id).Execute(); err != nil {
			return cmdutil.APIError("removing public interface", err)
		}
		cmd.Printf("Server %d's public interface removed.\n", id)
		return nil
	},
}

func init() {
	serverRescueEnterCmd.Flags().String("iso", "",
		"Boot ISO slug from `boot-isos` (omit for the default rescue image)")

	serverRescueCmd.AddCommand(serverRescueEnterCmd)
	serverRescueCmd.AddCommand(serverRescueExitCmd)

	serverPublicInterfaceSetCmd.Flags().StringVar(&serverPublicInterfaceFirewall, "firewall", "",
		"Firewall rule set, by ID or name")
	serverPublicInterfaceSetCmd.Flags().StringVar(&serverPublicInterfacePolicyIn, "policy-in", "",
		"Default inbound policy: ACCEPT, DROP or REJECT")
	serverPublicInterfaceSetCmd.Flags().StringVar(&serverPublicInterfacePolicyOut, "policy-out", "",
		"Default outbound policy: ACCEPT, DROP or REJECT")

	serverPublicInterfaceCmd.AddCommand(serverPublicInterfaceGetCmd)
	serverPublicInterfaceCmd.AddCommand(serverPublicInterfaceSetCmd)
	serverPublicInterfaceCmd.AddCommand(serverPublicInterfaceDeleteCmd)

	serverCmd.AddCommand(serverRescueCmd)
	serverCmd.AddCommand(serverBootISOsCmd)
	serverCmd.AddCommand(serverUsageCmd)
	serverCmd.AddCommand(serverActivityCmd)
	serverCmd.AddCommand(serverRetryProvisionCmd)
	serverCmd.AddCommand(serverPublicInterfaceCmd)
}
