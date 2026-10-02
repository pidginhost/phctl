package dedicated

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

var Cmd = &cobra.Command{
	Use:     "dedicated",
	Aliases: []string{"ded"},
	Short:   "Manage dedicated servers",
	Args:    cobra.NoArgs,
}

var serverCmd = &cobra.Command{
	Use:     "server",
	Aliases: []string{"s"},
	Short:   "Manage dedicated servers",
	Args:    cobra.NoArgs,
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all dedicated servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		servers, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.DedicatedServer, bool, error) {
			resp, _, err := c.DedicatedAPI.DedicatedServersList(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			if resp == nil {
				return nil, false, fmt.Errorf("server returned no dedicated server page")
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing dedicated servers", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, servers, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME", "STATUS", "SERVER STATUS", "IPS", "OS")
			for _, s := range servers {
				output.PrintRow(tw, s.Id, s.Hostname, s.Status, serverStatusText(s), serverIPs(s), output.Pstr(s.OsName.Get()))
			}
			tw.Flush()
		})
	},
}

var serverGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get dedicated server details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		s, _, err := c.DedicatedAPI.DedicatedServersRetrieve(cmd.Context(), args[0]).Execute()
		if err != nil {
			return cmdutil.APIError("getting dedicated server", err)
		}
		if s == nil {
			return fmt.Errorf("getting dedicated server %s: server returned no server", args[0])
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, s, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", s.Id)
			output.PrintRow(tw, "Hostname:", s.Hostname)
			output.PrintRow(tw, "Status:", s.Status)
			output.PrintRow(tw, "Server Status:", serverStatusText(*s))
			output.PrintRow(tw, "IPs:", serverIPs(*s))
			output.PrintRow(tw, "OS:", output.Pstr(s.OsName.Get()))
			output.PrintRow(tw, "Price:", s.Price)
			output.PrintRow(tw, "Billing Cycle:", s.BillingCycle)
			output.PrintRow(tw, "Next Invoice:", s.NextInvoice)
			tw.Flush()
		})
	},
}

// serverStatusText is the hardware state the provider reports, or <none>
// while it has not reported one.
func serverStatusText(s pidginhost.DedicatedServer) string {
	if st := s.ServerStatus.Get(); st != nil {
		return st.StatusText
	}
	return "<none>"
}

func serverIPs(s pidginhost.DedicatedServer) string {
	ips := make([]string, len(s.Ips))
	for i, ip := range s.Ips {
		ips[i] = ip.Ip
	}
	return strings.Join(ips, ", ")
}

var serverPowerAction string

var serverPowerCmd = &cobra.Command{
	Use:   "power <id>",
	Short: "Manage dedicated server power (--action start|stop|restart)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewPowerActionRequest(pidginhost.PowerActionActionEnum(serverPowerAction))
		resp, _, err := c.DedicatedAPI.DedicatedServersPowerCreate(cmd.Context(), args[0]).PowerActionRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("power management", err)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"Power action '%s': %s\n", serverPowerAction, resp.Message)
	},
}

var reinstallOSID int32

var serverReinstallCmd = &cobra.Command{
	Use:   "reinstall <id>",
	Short: "Reinstall OS on a dedicated server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Reinstall OS on dedicated server %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewReinstallRequest(reinstallOSID)
		_, _, err = c.DedicatedAPI.DedicatedServersReinstallCreate(cmd.Context(), args[0]).ReinstallRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("reinstalling", err)
		}
		cmd.Printf("OS reinstall queued for dedicated server %s.\n", args[0])
		return nil
	},
}

var (
	rdnsIPID     int32
	rdnsHostname string
)

var serverRDNSCmd = &cobra.Command{
	Use:   "rdns <id>",
	Short: "Configure reverse DNS for a dedicated server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewDedicatedRDNSRequest(rdnsIPID, rdnsHostname)
		resp, _, err := c.DedicatedAPI.DedicatedServersRdnsCreate(cmd.Context(), args[0]).DedicatedRDNSRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("setting rDNS", err)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"rDNS updated: %s\n", resp.Message)
	},
}

func init() {
	serverPowerCmd.Flags().StringVar(&serverPowerAction, "action", "", "Power action: start, stop, restart (required)")
	serverPowerCmd.MarkFlagRequired("action")

	serverReinstallCmd.Flags().Int32Var(&reinstallOSID, "os-id", 0, "OS template ID (required)")
	serverReinstallCmd.MarkFlagRequired("os-id")

	serverRDNSCmd.Flags().Int32Var(&rdnsIPID, "ip-id", 0, "IP address ID (required)")
	serverRDNSCmd.Flags().StringVar(&rdnsHostname, "hostname", "", "Reverse DNS hostname (required)")
	serverRDNSCmd.MarkFlagRequired("ip-id")
	serverRDNSCmd.MarkFlagRequired("hostname")

	serverCmd.AddCommand(serverListCmd)
	serverCmd.AddCommand(serverGetCmd)
	serverCmd.AddCommand(serverPowerCmd)
	serverCmd.AddCommand(serverReinstallCmd)
	serverCmd.AddCommand(serverRDNSCmd)

	Cmd.AddCommand(serverCmd)
}
