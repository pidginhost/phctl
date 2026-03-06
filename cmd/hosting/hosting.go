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

var (
	outputFormat = cmdutil.OutputFormat
)

var Cmd = &cobra.Command{
	Use:     "hosting",
	Aliases: []string{"host"},
	Short:   "Manage web hosting services",
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage hosting services",
}

var serviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List hosting services",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		services, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.HostingService, bool, error) {
			resp, _, err := c.HostingAPI.HostingHostingList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
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
		c, err := client.New()
		if err != nil {
			return err
		}
		s, _, err := c.HostingAPI.HostingHostingRetrieve(context.Background(), args[0]).Execute()
		if err != nil {
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
