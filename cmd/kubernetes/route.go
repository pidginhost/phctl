package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

// --- HTTP Routes ---

var httpRouteCmd = &cobra.Command{
	Use:   "http-route",
	Short: "Manage HTTP routes",
}

var httpRouteListCmd = &cobra.Command{
	Use:   "list <cluster-id>",
	Short: "List HTTP routes for a cluster",
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
		httpResp, err := c.KubernetesAPI.KubernetesClustersHttproutesRetrieve(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("listing HTTP routes: %w", err)
		}
		defer httpResp.Body.Close()

		var routes []pidginhost.HTTPRoute
		if err := json.NewDecoder(httpResp.Body).Decode(&routes); err != nil {
			return fmt.Errorf("decoding routes: %w", err)
		}

		format := outputFormat(cmd)
		output.Print(format, routes, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "HOSTNAMES", "BACKEND", "PORT", "TLS", "READY")
			for _, r := range routes {
				output.PrintRow(tw, r.Id, r.Name, r.Hostnames, r.BackendServiceName, r.BackendServicePort, pstr(r.EnableTls), pstr(r.StatusReady.Get()))
			}
			tw.Flush()
		})
		return nil
	},
}

var (
	httpRouteName      string
	httpRouteHostnames []string
	httpRouteBackend   string
	httpRoutePort      int32
	httpRouteNamespace string
	httpRoutePrefix    string
	httpRouteTLS       bool
)

var httpRouteCreateCmd = &cobra.Command{
	Use:   "create <cluster-id>",
	Short: "Create an HTTP route",
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
		body := *pidginhost.NewHTTPRoute(
			0, httpRouteName, httpRouteHostnames,
			httpRouteBackend, httpRoutePort,
			*pidginhost.NewNullableBool(nil),
			"",
			time.Time{}, time.Time{},
		)
		if httpRouteNamespace != "" {
			body.Namespace = pidginhost.PtrString(httpRouteNamespace)
		}
		if httpRoutePrefix != "" {
			body.PathPrefix = pidginhost.PtrString(httpRoutePrefix)
		}
		if httpRouteTLS {
			body.EnableTls = pidginhost.PtrBool(true)
		}

		resp, _, err := c.KubernetesAPI.KubernetesClustersHttproutesCreate(context.Background(), id).HTTPRoute(body).Execute()
		if err != nil {
			return fmt.Errorf("creating HTTP route: %w", err)
		}
		fmt.Printf("HTTP route created (ID: %d, Name: %s)\n", resp.Id, resp.Name)
		return nil
	},
}

var httpRouteDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <route-id>",
	Aliases: []string{"rm"},
	Short:   "Delete an HTTP route",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete HTTP route %s?", args[1])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersHttproutesDestroy(context.Background(), id, args[1]).Execute()
		if err != nil {
			return fmt.Errorf("deleting HTTP route: %w", err)
		}
		fmt.Printf("HTTP route %s deleted.\n", args[1])
		return nil
	},
}

// --- TCP Routes ---

var tcpRouteCmd = &cobra.Command{
	Use:   "tcp-route",
	Short: "Manage TCP routes",
}

var tcpRouteListCmd = &cobra.Command{
	Use:   "list <cluster-id>",
	Short: "List TCP routes for a cluster",
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
		httpResp, err := c.KubernetesAPI.KubernetesClustersTcproutesRetrieve(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("listing TCP routes: %w", err)
		}
		defer httpResp.Body.Close()

		var routes []pidginhost.TCPRoute
		if err := json.NewDecoder(httpResp.Body).Decode(&routes); err != nil {
			return fmt.Errorf("decoding routes: %w", err)
		}

		format := outputFormat(cmd)
		output.Print(format, routes, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "PORT", "BACKEND", "BACKEND PORT", "READY")
			for _, r := range routes {
				output.PrintRow(tw, r.Id, r.Name, r.Port, r.BackendServiceName, r.BackendServicePort, pstr(r.StatusReady.Get()))
			}
			tw.Flush()
		})
		return nil
	},
}

var (
	tcpRouteName      string
	tcpRoutePort      int32
	tcpRouteBackend   string
	tcpRouteBackPort  int32
	tcpRouteNamespace string
)

var tcpRouteCreateCmd = &cobra.Command{
	Use:   "create <cluster-id>",
	Short: "Create a TCP route",
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
		body := *pidginhost.NewTCPRoute(
			0, tcpRouteName, tcpRoutePort,
			tcpRouteBackend, tcpRouteBackPort,
			*pidginhost.NewNullableBool(nil),
			"",
			time.Time{}, time.Time{},
		)
		if tcpRouteNamespace != "" {
			body.Namespace = pidginhost.PtrString(tcpRouteNamespace)
		}

		resp, _, err := c.KubernetesAPI.KubernetesClustersTcproutesCreate(context.Background(), id).TCPRoute(body).Execute()
		if err != nil {
			return fmt.Errorf("creating TCP route: %w", err)
		}
		fmt.Printf("TCP route created (ID: %d, Name: %s)\n", resp.Id, resp.Name)
		return nil
	},
}

var tcpRouteDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <route-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a TCP route",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete TCP route %s?", args[1])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersTcproutesDestroy(context.Background(), id, args[1]).Execute()
		if err != nil {
			return fmt.Errorf("deleting TCP route: %w", err)
		}
		fmt.Printf("TCP route %s deleted.\n", args[1])
		return nil
	},
}

// --- UDP Routes ---

var udpRouteCmd = &cobra.Command{
	Use:   "udp-route",
	Short: "Manage UDP routes",
}

var udpRouteListCmd = &cobra.Command{
	Use:   "list <cluster-id>",
	Short: "List UDP routes for a cluster",
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
		httpResp, err := c.KubernetesAPI.KubernetesClustersUdproutesRetrieve(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("listing UDP routes: %w", err)
		}
		defer httpResp.Body.Close()

		var routes []pidginhost.UDPRoute
		if err := json.NewDecoder(httpResp.Body).Decode(&routes); err != nil {
			return fmt.Errorf("decoding routes: %w", err)
		}

		format := outputFormat(cmd)
		output.Print(format, routes, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "PORT", "BACKEND", "BACKEND PORT", "READY")
			for _, r := range routes {
				output.PrintRow(tw, r.Id, r.Name, r.Port, r.BackendServiceName, r.BackendServicePort, pstr(r.StatusReady.Get()))
			}
			tw.Flush()
		})
		return nil
	},
}

var (
	udpRouteName      string
	udpRoutePort      int32
	udpRouteBackend   string
	udpRouteBackPort  int32
	udpRouteNamespace string
)

var udpRouteCreateCmd = &cobra.Command{
	Use:   "create <cluster-id>",
	Short: "Create a UDP route",
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
		body := *pidginhost.NewUDPRoute(
			0, udpRouteName, udpRoutePort,
			udpRouteBackend, udpRouteBackPort,
			*pidginhost.NewNullableBool(nil),
			"",
			time.Time{}, time.Time{},
		)
		if udpRouteNamespace != "" {
			body.Namespace = pidginhost.PtrString(udpRouteNamespace)
		}

		resp, _, err := c.KubernetesAPI.KubernetesClustersUdproutesCreate(context.Background(), id).UDPRoute(body).Execute()
		if err != nil {
			return fmt.Errorf("creating UDP route: %w", err)
		}
		fmt.Printf("UDP route created (ID: %d, Name: %s)\n", resp.Id, resp.Name)
		return nil
	},
}

var udpRouteDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <route-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a UDP route",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete UDP route %s?", args[1])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersUdproutesDestroy(context.Background(), id, args[1]).Execute()
		if err != nil {
			return fmt.Errorf("deleting UDP route: %w", err)
		}
		fmt.Printf("UDP route %s deleted.\n", args[1])
		return nil
	},
}

func init() {
	httpRouteCreateCmd.Flags().StringVar(&httpRouteName, "name", "", "Route name (required)")
	httpRouteCreateCmd.Flags().StringSliceVar(&httpRouteHostnames, "hostname", nil, "Hostnames (required, can specify multiple)")
	httpRouteCreateCmd.Flags().StringVar(&httpRouteBackend, "backend", "", "Backend service name (required)")
	httpRouteCreateCmd.Flags().Int32Var(&httpRoutePort, "port", 0, "Backend service port (required)")
	httpRouteCreateCmd.Flags().StringVar(&httpRouteNamespace, "namespace", "", "Namespace")
	httpRouteCreateCmd.Flags().StringVar(&httpRoutePrefix, "path-prefix", "", "Path prefix (default: /)")
	httpRouteCreateCmd.Flags().BoolVar(&httpRouteTLS, "tls", false, "Enable TLS with auto cert issuance")
	httpRouteCreateCmd.MarkFlagRequired("name")
	httpRouteCreateCmd.MarkFlagRequired("hostname")
	httpRouteCreateCmd.MarkFlagRequired("backend")
	httpRouteCreateCmd.MarkFlagRequired("port")

	tcpRouteCreateCmd.Flags().StringVar(&tcpRouteName, "name", "", "Route name (required)")
	tcpRouteCreateCmd.Flags().Int32Var(&tcpRoutePort, "port", 0, "External port (required)")
	tcpRouteCreateCmd.Flags().StringVar(&tcpRouteBackend, "backend", "", "Backend service name (required)")
	tcpRouteCreateCmd.Flags().Int32Var(&tcpRouteBackPort, "backend-port", 0, "Backend service port (required)")
	tcpRouteCreateCmd.Flags().StringVar(&tcpRouteNamespace, "namespace", "", "Namespace")
	tcpRouteCreateCmd.MarkFlagRequired("name")
	tcpRouteCreateCmd.MarkFlagRequired("port")
	tcpRouteCreateCmd.MarkFlagRequired("backend")
	tcpRouteCreateCmd.MarkFlagRequired("backend-port")

	udpRouteCreateCmd.Flags().StringVar(&udpRouteName, "name", "", "Route name (required)")
	udpRouteCreateCmd.Flags().Int32Var(&udpRoutePort, "port", 0, "External port (required)")
	udpRouteCreateCmd.Flags().StringVar(&udpRouteBackend, "backend", "", "Backend service name (required)")
	udpRouteCreateCmd.Flags().Int32Var(&udpRouteBackPort, "backend-port", 0, "Backend service port (required)")
	udpRouteCreateCmd.Flags().StringVar(&udpRouteNamespace, "namespace", "", "Namespace")
	udpRouteCreateCmd.MarkFlagRequired("name")
	udpRouteCreateCmd.MarkFlagRequired("port")
	udpRouteCreateCmd.MarkFlagRequired("backend")
	udpRouteCreateCmd.MarkFlagRequired("backend-port")

	httpRouteCmd.AddCommand(httpRouteListCmd)
	httpRouteCmd.AddCommand(httpRouteCreateCmd)
	httpRouteCmd.AddCommand(httpRouteDeleteCmd)

	tcpRouteCmd.AddCommand(tcpRouteListCmd)
	tcpRouteCmd.AddCommand(tcpRouteCreateCmd)
	tcpRouteCmd.AddCommand(tcpRouteDeleteCmd)

	udpRouteCmd.AddCommand(udpRouteListCmd)
	udpRouteCmd.AddCommand(udpRouteCreateCmd)
	udpRouteCmd.AddCommand(udpRouteDeleteCmd)

	Cmd.AddCommand(httpRouteCmd)
	Cmd.AddCommand(tcpRouteCmd)
	Cmd.AddCommand(udpRouteCmd)
}
