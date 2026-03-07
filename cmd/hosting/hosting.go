package hosting

import (
	"context"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/output"
)

type rawHostingService struct {
	Id           int32  `json:"id"`
	Hostname     string `json:"hostname"`
	Status       string `json:"status"`
	Price        string `json:"price"`
	NextInvoice  string `json:"next_invoice"`
	Created      string `json:"created"`
	BillingCycle string `json:"billing_cycle"`
	PackageName  string `json:"package_name"`
	NodeUrl      string `json:"node_url"`
	Username     string `json:"username"`
}

var (
	outputFormat = cmdutil.OutputFormat
)

var Cmd = &cobra.Command{
	Use:     "hosting",
	Aliases: []string{"host"},
	Short:   "Manage web hosting services",
	Args:    cobra.NoArgs,
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage hosting services",
	Args:  cobra.NoArgs,
}

var serviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List hosting services",
	RunE: func(cmd *cobra.Command, args []string) error {
		services, err := client.RawFetchAll[rawHostingService]("/api/hosting/hosting/")
		if err != nil {
			return fmt.Errorf("listing hosting services: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, services, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME", "PACKAGE", "STATUS", "USERNAME")
			for _, s := range services {
				output.PrintRow(tw, s.Id, s.Hostname, s.PackageName, s.Status, s.Username)
			}
			tw.Flush()
		})
	},
}

var serviceGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get hosting service details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var s rawHostingService
		if err := client.RawGet(fmt.Sprintf("/api/hosting/hosting/%s/", args[0]), &s); err != nil {
			return fmt.Errorf("getting hosting service: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, s, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", s.Id)
			output.PrintRow(tw, "Hostname:", s.Hostname)
			output.PrintRow(tw, "Package:", s.PackageName)
			output.PrintRow(tw, "Status:", s.Status)
			output.PrintRow(tw, "Username:", s.Username)
			output.PrintRow(tw, "Node URL:", s.NodeUrl)
			output.PrintRow(tw, "Price:", s.Price)
			output.PrintRow(tw, "Billing Cycle:", s.BillingCycle)
			tw.Flush()
		})
	},
}

var changePasswordNew string

var changePasswordCmd = &cobra.Command{
	Use:   "change-password <id>",
	Short: "Change cPanel password for a hosting service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewChangePassword(changePasswordNew)
		resp, _, err := c.HostingAPI.HostingHostingChangePasswordCreate(context.Background(), args[0]).ChangePassword(body).Execute()
		if err != nil {
			return fmt.Errorf("changing password: %w", err)
		}
		fmt.Printf("Password changed: %s\n", resp.Message)
		return nil
	},
}

func init() {
	changePasswordCmd.Flags().StringVar(&changePasswordNew, "password", "", "New password (required)")
	changePasswordCmd.MarkFlagRequired("password")

	serviceCmd.AddCommand(serviceListCmd)
	serviceCmd.AddCommand(serviceGetCmd)

	Cmd.AddCommand(serviceCmd)
	Cmd.AddCommand(changePasswordCmd)
}
