package dedicated

import (
	"context"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var (
	outputFormat = cmdutil.OutputFormat
	force        = cmdutil.Force
)

var Cmd = &cobra.Command{
	Use:     "dedicated",
	Aliases: []string{"ded"},
	Short:   "Manage dedicated servers",
}

var serverCmd = &cobra.Command{
	Use:     "server",
	Aliases: []string{"s"},
	Short:   "Manage dedicated servers",
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
			resp, _, err := c.DedicatedAPI.DedicatedServersList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing dedicated servers: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, servers, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME", "STATUS", "SERVER STATUS", "IPS", "OS")
			for _, s := range servers {
				output.PrintRow(tw, s.Id, s.Hostname, s.Status, s.ServerStatus, s.Ips, s.OsName)
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
		s, _, err := c.DedicatedAPI.DedicatedServersRetrieve(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("getting dedicated server: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, s, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", s.Id)
			output.PrintRow(tw, "Hostname:", s.Hostname)
			output.PrintRow(tw, "Status:", s.Status)
			output.PrintRow(tw, "Server Status:", s.ServerStatus)
			output.PrintRow(tw, "IPs:", s.Ips)
			output.PrintRow(tw, "OS:", s.OsName)
			output.PrintRow(tw, "Price:", s.Price)
			output.PrintRow(tw, "Billing Cycle:", s.BillingCycle)
			output.PrintRow(tw, "Next Invoice:", s.NextInvoice)
			tw.Flush()
		})
	},
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
		body := *pidginhost.NewPowerAction(pidginhost.PowerActionActionEnum(serverPowerAction))
		resp, _, err := c.DedicatedAPI.DedicatedServersPowerCreate(context.Background(), args[0]).PowerAction(body).Execute()
		if err != nil {
			return fmt.Errorf("power management: %w", err)
		}
		fmt.Printf("Power action '%s': %s\n", serverPowerAction, resp.Message)
		return nil
	},
}

var reinstallOSID int32

var serverReinstallCmd = &cobra.Command{
	Use:   "reinstall <id>",
	Short: "Reinstall OS on a dedicated server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Reinstall OS on dedicated server %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewReinstall(reinstallOSID)
		_, _, err = c.DedicatedAPI.DedicatedServersReinstallCreate(context.Background(), args[0]).Reinstall(body).Execute()
		if err != nil {
			return fmt.Errorf("reinstalling: %w", err)
		}
		fmt.Printf("OS reinstall queued for dedicated server %s.\n", args[0])
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
		body := *pidginhost.NewDedicatedRDNS(rdnsIPID, rdnsHostname)
		resp, _, err := c.DedicatedAPI.DedicatedServersRdnsCreate(context.Background(), args[0]).DedicatedRDNS(body).Execute()
		if err != nil {
			return fmt.Errorf("setting rDNS: %w", err)
		}
		fmt.Printf("rDNS updated: %s\n", resp.Message)
		return nil
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
