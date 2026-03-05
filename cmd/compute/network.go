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

var networkCmd = &cobra.Command{
	Use:     "network",
	Aliases: []string{"net"},
	Short:   "Manage private networks",
}

var networkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all private networks",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.CloudAPI.CloudPrivateNetworksList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing networks: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, resp.Results, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "SLUG", "ADDRESS", "PROVISIONED", "SERVERS")
			for _, n := range resp.Results {
				output.PrintRow(tw, n.Id, n.Slug, n.Address, n.Provisioned, len(n.Servers))
			}
			tw.Flush()
		})
		return nil
	},
}

var networkGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get private network details",
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
		net, _, err := c.CloudAPI.CloudPrivateNetworksRetrieve(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("getting network: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, net, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", net.Id)
			output.PrintRow(tw, "Slug:", net.Slug)
			output.PrintRow(tw, "Address:", net.Address)
			output.PrintRow(tw, "Provisioned:", net.Provisioned)
			tw.Flush()
			if len(net.Servers) > 0 {
				fmt.Fprintln(w)
				fmt.Fprintln(w, "Servers:")
				for _, s := range net.Servers {
					for k, v := range s {
						fmt.Fprintf(w, "  %s: %s\n", k, v)
					}
				}
			}
		})
		return nil
	},
}

var networkCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a private network",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewPrivateNetwork(0, "", "", false, nil)
		resp, _, err := c.CloudAPI.CloudPrivateNetworksCreate(context.Background()).PrivateNetwork(body).Execute()
		if err != nil {
			return fmt.Errorf("creating network: %w", err)
		}
		fmt.Printf("Private network created (ID: %d, Address: %s)\n", resp.Id, resp.Address)
		return nil
	},
}

var networkDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a private network",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete private network %d?", id)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.CloudAPI.CloudPrivateNetworksDestroy(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("deleting network: %w", err)
		}
		fmt.Printf("Private network %d deleted.\n", id)
		return nil
	},
}

var (
	networkAddServerHost    string
	networkAddServerAddress string
)

var networkAddServerCmd = &cobra.Command{
	Use:   "add-server <network-id>",
	Short: "Add a server to a private network",
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
		body := *pidginhost.NewPrivateNetworkAddHost(networkAddServerHost)
		if networkAddServerAddress != "" {
			body.Address = pidginhost.PtrString(networkAddServerAddress)
		}
		resp, _, err := c.CloudAPI.CloudPrivateNetworksAddServerCreate(context.Background(), id).PrivateNetworkAddHost(body).Execute()
		if err != nil {
			return fmt.Errorf("adding server to network: %w", err)
		}
		fmt.Printf("Server added to network: %v\n", resp.Created)
		return nil
	},
}

var networkRemoveServerHost string

var networkRemoveServerCmd = &cobra.Command{
	Use:   "remove-server <network-id>",
	Short: "Remove a server from a private network",
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
		body := *pidginhost.NewPrivateNetworkRemoveHost(networkRemoveServerHost)
		resp, _, err := c.CloudAPI.CloudPrivateNetworksRemoveServerCreate(context.Background(), id).PrivateNetworkRemoveHost(body).Execute()
		if err != nil {
			return fmt.Errorf("removing server from network: %w", err)
		}
		fmt.Printf("Server removed from network: %v\n", resp.Removed)
		return nil
	},
}

func init() {
	networkAddServerCmd.Flags().StringVar(&networkAddServerHost, "server", "", "Server hostname (required)")
	networkAddServerCmd.Flags().StringVar(&networkAddServerAddress, "address", "", "Private IP address to assign")
	networkAddServerCmd.MarkFlagRequired("server")

	networkRemoveServerCmd.Flags().StringVar(&networkRemoveServerHost, "server", "", "Server hostname or private IP (required)")
	networkRemoveServerCmd.MarkFlagRequired("server")

	networkCmd.AddCommand(networkListCmd)
	networkCmd.AddCommand(networkGetCmd)
	networkCmd.AddCommand(networkCreateCmd)
	networkCmd.AddCommand(networkDeleteCmd)
	networkCmd.AddCommand(networkAddServerCmd)
	networkCmd.AddCommand(networkRemoveServerCmd)
}
