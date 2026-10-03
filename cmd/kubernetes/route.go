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

// --- HTTP Routes ---

var httpRouteCmd = &cobra.Command{
	Use:   "http-route",
	Short: "Manage HTTP routes",
	Args:  cobra.NoArgs,
}

var httpRouteListCmd = &cobra.Command{
	Use:   "list <cluster-id>",
	Short: "List HTTP routes for a cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		routes, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.HTTPRoute, bool, error) {
			resp, _, err := c.KubernetesAPI.KubernetesClustersHttproutesList(cmd.Context(), id).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			if resp == nil {
				return nil, false, fmt.Errorf("server returned no route list")
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing HTTP routes", err)
		}

		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, routes, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "HOSTNAMES", "BACKEND", "PORT", "TLS", "READY")
			for _, r := range routes {
				output.PrintRow(tw, r.Id, r.Name, r.Hostnames, r.BackendServiceName, r.BackendServicePort, output.Pstr(r.EnableTls), output.Pstr(r.StatusReady.Get()))
			}
			tw.Flush()
		})
	},
}

var (
	httpRouteName      string
	httpRouteHostnames []string
	httpRouteBackend   string
	httpRoutePort      int32
	httpRouteNamespace string
	httpRouteBackendNS string
	httpRoutePrefix    string
	httpRouteTLS       bool
)

var httpRouteCreateCmd = &cobra.Command{
	Use:   "create <cluster-id>",
	Short: "Create an HTTP route",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewHTTPRouteRequest(
			httpRouteName, httpRouteHostnames,
			httpRouteBackend, httpRoutePort,
		)
		if httpRouteNamespace != "" {
			body.Namespace = pidginhost.PtrString(httpRouteNamespace)
		}
		if httpRouteBackendNS != "" {
			body.BackendNamespace = pidginhost.PtrString(httpRouteBackendNS)
		}
		if httpRoutePrefix != "" {
			body.PathPrefix = pidginhost.PtrString(httpRoutePrefix)
		}
		body.EnableTls = pidginhost.PtrBool(httpRouteTLS)

		resp, _, err := c.KubernetesAPI.KubernetesClustersHttproutesCreate(cmd.Context(), id).HTTPRouteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating HTTP route", err)
		}
		if resp == nil {
			return fmt.Errorf("creating HTTP route: server returned no route")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"HTTP route created (ID: %d, Name: %s)\n", resp.Id, resp.Name)
	},
}

var httpRouteDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <route-id>",
	Aliases: []string{"rm"},
	Short:   "Delete an HTTP route",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if _, err := cmdutil.ParseInt32(args[1]); err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete HTTP route %s?", args[1])) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersHttproutesDestroy(cmd.Context(), id, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("deleting HTTP route", err)
		}
		cmd.Printf("HTTP route %s deleted.\n", args[1])
		return nil
	},
}

// --- TCP Routes ---

var tcpRouteCmd = &cobra.Command{
	Use:   "tcp-route",
	Short: "Manage TCP routes",
	Args:  cobra.NoArgs,
}

var tcpRouteListCmd = &cobra.Command{
	Use:   "list <cluster-id>",
	Short: "List TCP routes for a cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		routes, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.TCPRoute, bool, error) {
			resp, _, err := c.KubernetesAPI.KubernetesClustersTcproutesList(cmd.Context(), id).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			if resp == nil {
				return nil, false, fmt.Errorf("server returned no route list")
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing TCP routes", err)
		}

		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, routes, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "PORT", "BACKEND", "BACKEND PORT", "READY")
			for _, r := range routes {
				output.PrintRow(tw, r.Id, r.Name, r.Port, r.BackendServiceName, r.BackendServicePort, output.Pstr(r.StatusReady.Get()))
			}
			tw.Flush()
		})
	},
}

var (
	tcpRouteName      string
	tcpRoutePort      int32
	tcpRouteBackend   string
	tcpRouteBackPort  int32
	tcpRouteNamespace string
	tcpRouteBackendNS string
)

var tcpRouteCreateCmd = &cobra.Command{
	Use:   "create <cluster-id>",
	Short: "Create a TCP route",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewTCPRouteRequest(
			tcpRouteName, tcpRoutePort,
			tcpRouteBackend, tcpRouteBackPort,
		)
		if tcpRouteNamespace != "" {
			body.Namespace = pidginhost.PtrString(tcpRouteNamespace)
		}
		if tcpRouteBackendNS != "" {
			body.BackendNamespace = pidginhost.PtrString(tcpRouteBackendNS)
		}

		resp, _, err := c.KubernetesAPI.KubernetesClustersTcproutesCreate(cmd.Context(), id).TCPRouteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating TCP route", err)
		}
		if resp == nil {
			return fmt.Errorf("creating TCP route: server returned no route")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"TCP route created (ID: %d, Name: %s)\n", resp.Id, resp.Name)
	},
}

var tcpRouteDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <route-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a TCP route",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if _, err := cmdutil.ParseInt32(args[1]); err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete TCP route %s?", args[1])) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersTcproutesDestroy(cmd.Context(), id, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("deleting TCP route", err)
		}
		cmd.Printf("TCP route %s deleted.\n", args[1])
		return nil
	},
}

// --- UDP Routes ---

var udpRouteCmd = &cobra.Command{
	Use:   "udp-route",
	Short: "Manage UDP routes",
	Args:  cobra.NoArgs,
}

var udpRouteListCmd = &cobra.Command{
	Use:   "list <cluster-id>",
	Short: "List UDP routes for a cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		routes, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.UDPRoute, bool, error) {
			resp, _, err := c.KubernetesAPI.KubernetesClustersUdproutesList(cmd.Context(), id).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			if resp == nil {
				return nil, false, fmt.Errorf("server returned no route list")
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing UDP routes", err)
		}

		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, routes, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "PORT", "BACKEND", "BACKEND PORT", "READY")
			for _, r := range routes {
				output.PrintRow(tw, r.Id, r.Name, r.Port, r.BackendServiceName, r.BackendServicePort, output.Pstr(r.StatusReady.Get()))
			}
			tw.Flush()
		})
	},
}

var (
	udpRouteName      string
	udpRoutePort      int32
	udpRouteBackend   string
	udpRouteBackPort  int32
	udpRouteNamespace string
	udpRouteBackendNS string
)

var udpRouteCreateCmd = &cobra.Command{
	Use:   "create <cluster-id>",
	Short: "Create a UDP route",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewUDPRouteRequest(
			udpRouteName, udpRoutePort,
			udpRouteBackend, udpRouteBackPort,
		)
		if udpRouteNamespace != "" {
			body.Namespace = pidginhost.PtrString(udpRouteNamespace)
		}
		if udpRouteBackendNS != "" {
			body.BackendNamespace = pidginhost.PtrString(udpRouteBackendNS)
		}

		resp, _, err := c.KubernetesAPI.KubernetesClustersUdproutesCreate(cmd.Context(), id).UDPRouteRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating UDP route", err)
		}
		if resp == nil {
			return fmt.Errorf("creating UDP route: server returned no route")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"UDP route created (ID: %d, Name: %s)\n", resp.Id, resp.Name)
	},
}

var udpRouteDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <route-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a UDP route",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if _, err := cmdutil.ParseInt32(args[1]); err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete UDP route %s?", args[1])) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersUdproutesDestroy(cmd.Context(), id, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("deleting UDP route", err)
		}
		cmd.Printf("UDP route %s deleted.\n", args[1])
		return nil
	},
}

func init() {
	httpRouteCreateCmd.Flags().StringVar(&httpRouteName, "name", "", "Route name (required)")
	httpRouteCreateCmd.Flags().StringSliceVar(&httpRouteHostnames, "hostname", nil, "Hostnames (required, can specify multiple)")
	httpRouteCreateCmd.Flags().StringVar(&httpRouteBackend, "backend", "", "Backend service name (required)")
	httpRouteCreateCmd.Flags().Int32Var(&httpRoutePort, "port", 0, "Backend service port (required)")
	httpRouteCreateCmd.Flags().StringVar(&httpRouteNamespace, "namespace", "", "Namespace")
	httpRouteCreateCmd.Flags().StringVar(&httpRouteBackendNS, "backend-namespace", "", "Backend service namespace (default: default)")
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
	tcpRouteCreateCmd.Flags().StringVar(&tcpRouteBackendNS, "backend-namespace", "", "Backend service namespace (default: default)")
	tcpRouteCreateCmd.MarkFlagRequired("name")
	tcpRouteCreateCmd.MarkFlagRequired("port")
	tcpRouteCreateCmd.MarkFlagRequired("backend")
	tcpRouteCreateCmd.MarkFlagRequired("backend-port")

	udpRouteCreateCmd.Flags().StringVar(&udpRouteName, "name", "", "Route name (required)")
	udpRouteCreateCmd.Flags().Int32Var(&udpRoutePort, "port", 0, "External port (required)")
	udpRouteCreateCmd.Flags().StringVar(&udpRouteBackend, "backend", "", "Backend service name (required)")
	udpRouteCreateCmd.Flags().Int32Var(&udpRouteBackPort, "backend-port", 0, "Backend service port (required)")
	udpRouteCreateCmd.Flags().StringVar(&udpRouteNamespace, "namespace", "", "Namespace")
	udpRouteCreateCmd.Flags().StringVar(&udpRouteBackendNS, "backend-namespace", "", "Backend service namespace (default: default)")
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
