package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/output"
)

// Gateway route detail (get) and replace (update).
//
// The detail routes carry no response schema -- the viewsets override
// get_serializer to inject the cluster, which drf-spectacular cannot do, so
// only the write methods (which declare their serializer explicitly) are typed.
// list already decodes the body by hand for the same reason; get follows it.
//
// update is PUT rather than PATCH on purpose: the server rebuilds the route
// manifest from the request body, so a partial body has nothing to rebuild
// from. Every field is therefore required, and the command replaces the route.

// decodeRouteDetail reads an untyped detail response into dst.
func decodeRouteDetail(httpResp *http.Response, err error, op string, dst any) error {
	if err != nil {
		return cmdutil.APIError(op, err)
	}
	if httpResp == nil || httpResp.Body == nil {
		return fmt.Errorf("%s: server returned no response body", op)
	}
	defer httpResp.Body.Close()
	if err := json.NewDecoder(httpResp.Body).Decode(dst); err != nil {
		return fmt.Errorf("%s: decoding response: %w", op, err)
	}
	return nil
}

func printHTTPRoute(w io.Writer, r *pidginhost.HTTPRoute) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "ID:", r.Id)
	output.PrintRow(tw, "Name:", r.Name)
	output.PrintRow(tw, "Namespace:", output.Pstr(r.Namespace))
	output.PrintRow(tw, "Hostnames:", r.Hostnames)
	output.PrintRow(tw, "Backend service:", r.BackendServiceName)
	output.PrintRow(tw, "Backend port:", r.BackendServicePort)
	output.PrintRow(tw, "Backend namespace:", output.Pstr(r.BackendNamespace))
	output.PrintRow(tw, "Path prefix:", output.Pstr(r.PathPrefix))
	output.PrintRow(tw, "TLS:", output.Pstr(r.EnableTls))
	output.PrintRow(tw, "Ready:", output.Pstr(r.StatusReady.Get()))
	output.PrintRow(tw, "Status:", r.StatusMessage)
	output.PrintRow(tw, "Created:", r.Created)
	output.PrintRow(tw, "Updated:", r.Updated)
	tw.Flush()
}

// printPortRoute renders a TCP or UDP route. Both carry the same fields.
func printPortRoute(w io.Writer, id int32, name string, namespace *string, port int32,
	backend string, backendPort int32, backendNamespace *string,
	ready *bool, statusMessage, created, updated string) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "ID:", id)
	output.PrintRow(tw, "Name:", name)
	output.PrintRow(tw, "Namespace:", output.Pstr(namespace))
	output.PrintRow(tw, "Port:", port)
	output.PrintRow(tw, "Backend service:", backend)
	output.PrintRow(tw, "Backend port:", backendPort)
	output.PrintRow(tw, "Backend namespace:", output.Pstr(backendNamespace))
	output.PrintRow(tw, "Ready:", output.Pstr(ready))
	output.PrintRow(tw, "Status:", statusMessage)
	output.PrintRow(tw, "Created:", created)
	output.PrintRow(tw, "Updated:", updated)
	tw.Flush()
}

// --- get ---

var httpRouteGetCmd = &cobra.Command{
	Use:   "get <cluster-id> <route-id>",
	Short: "Get an HTTP route",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, routeID, err := parseRouteArgs(args)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		var route pidginhost.HTTPRoute
		httpResp, err := c.KubernetesAPI.KubernetesClustersHttproutesRetrieve2(cmd.Context(), clusterID, args[1]).Execute()
		if err := decodeRouteDetail(httpResp, err, "getting HTTP route", &route); err != nil {
			return err
		}
		if route.Id != routeID {
			return fmt.Errorf("getting HTTP route %d: server returned route %d", routeID, route.Id)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route, func(w io.Writer) {
			printHTTPRoute(w, &route)
		})
	},
}

var tcpRouteGetCmd = &cobra.Command{
	Use:   "get <cluster-id> <route-id>",
	Short: "Get a TCP route",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, routeID, err := parseRouteArgs(args)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		var route pidginhost.TCPRoute
		httpResp, err := c.KubernetesAPI.KubernetesClustersTcproutesRetrieve2(cmd.Context(), clusterID, args[1]).Execute()
		if err := decodeRouteDetail(httpResp, err, "getting TCP route", &route); err != nil {
			return err
		}
		if route.Id != routeID {
			return fmt.Errorf("getting TCP route %d: server returned route %d", routeID, route.Id)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route, func(w io.Writer) {
			printPortRoute(w, route.Id, route.Name, route.Namespace, route.Port,
				route.BackendServiceName, route.BackendServicePort, route.BackendNamespace,
				route.StatusReady.Get(), route.StatusMessage, route.Created, route.Updated)
		})
	},
}

var udpRouteGetCmd = &cobra.Command{
	Use:   "get <cluster-id> <route-id>",
	Short: "Get a UDP route",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, routeID, err := parseRouteArgs(args)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		var route pidginhost.UDPRoute
		httpResp, err := c.KubernetesAPI.KubernetesClustersUdproutesRetrieve2(cmd.Context(), clusterID, args[1]).Execute()
		if err := decodeRouteDetail(httpResp, err, "getting UDP route", &route); err != nil {
			return err
		}
		if route.Id != routeID {
			return fmt.Errorf("getting UDP route %d: server returned route %d", routeID, route.Id)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route, func(w io.Writer) {
			printPortRoute(w, route.Id, route.Name, route.Namespace, route.Port,
				route.BackendServiceName, route.BackendServicePort, route.BackendNamespace,
				route.StatusReady.Get(), route.StatusMessage, route.Created, route.Updated)
		})
	},
}

// --- update ---

var (
	httpRouteUpdateName      string
	httpRouteUpdateHostnames []string
	httpRouteUpdateBackend   string
	httpRouteUpdatePort      int32
	httpRouteUpdateNamespace string
	httpRouteUpdatePrefix    string
	httpRouteUpdateTLS       bool
)

const routeUpdateLong = "The server rebuilds the route from the request body, so this replaces the\n" +
	"route rather than patching it: pass every field you want the route to have.\n" +
	"Updating re-applies the route to the cluster."

var httpRouteUpdateCmd = &cobra.Command{
	Use:   "update <cluster-id> <route-id>",
	Short: "Replace an HTTP route",
	Long:  "Replace an HTTP route.\n\n" + routeUpdateLong,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, routeID, err := parseRouteArgs(args)
		if err != nil {
			return err
		}
		if httpRouteUpdateName == "" {
			return fmt.Errorf("--name is required")
		}
		if len(httpRouteUpdateHostnames) == 0 {
			return fmt.Errorf("--hostname is required (repeat it for several hostnames)")
		}
		if httpRouteUpdateBackend == "" {
			return fmt.Errorf("--backend is required")
		}
		if httpRouteUpdatePort == 0 {
			return fmt.Errorf("--port is required")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewHTTPRoute(
			routeID, httpRouteUpdateName, httpRouteUpdateHostnames,
			httpRouteUpdateBackend, httpRouteUpdatePort,
			*pidginhost.NewNullableBool(nil),
			"", "", "",
		)
		if httpRouteUpdateNamespace != "" {
			body.Namespace = pidginhost.PtrString(httpRouteUpdateNamespace)
		}
		if httpRouteUpdatePrefix != "" {
			body.PathPrefix = pidginhost.PtrString(httpRouteUpdatePrefix)
		}
		body.EnableTls = pidginhost.PtrBool(httpRouteUpdateTLS)

		route, _, err := c.KubernetesAPI.KubernetesClustersHttproutesUpdate(cmd.Context(), clusterID, args[1]).
			HTTPRoute(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating HTTP route", err)
		}
		if route == nil {
			return fmt.Errorf("updating HTTP route %d: server returned no route", routeID)
		}
		if route.Id != routeID {
			return fmt.Errorf("updating HTTP route %d: server returned route %d", routeID, route.Id)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route,
			"HTTP route %d updated (Name: %s).\n", route.Id, route.Name)
	},
}

// portRouteFields are the writable fields TCP and UDP routes share.
type portRouteFields struct {
	name        string
	port        int32
	backend     string
	backendPort int32
	namespace   string
}

func (f *portRouteFields) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Route name (required)")
	cmd.Flags().Int32Var(&f.port, "port", 0, "External port to expose (required)")
	cmd.Flags().StringVar(&f.backend, "backend", "", "Backend service name (required)")
	cmd.Flags().Int32Var(&f.backendPort, "backend-port", 0, "Backend service port (required)")
	cmd.Flags().StringVar(&f.namespace, "namespace", "", "Kubernetes namespace")
}

func (f *portRouteFields) validate() error {
	if f.name == "" {
		return fmt.Errorf("--name is required")
	}
	if f.port == 0 {
		return fmt.Errorf("--port is required")
	}
	if f.backend == "" {
		return fmt.Errorf("--backend is required")
	}
	if f.backendPort == 0 {
		return fmt.Errorf("--backend-port is required")
	}
	return nil
}

var (
	tcpRouteUpdateFields portRouteFields
	udpRouteUpdateFields portRouteFields
)

var tcpRouteUpdateCmd = &cobra.Command{
	Use:   "update <cluster-id> <route-id>",
	Short: "Replace a TCP route",
	Long:  "Replace a TCP route.\n\n" + routeUpdateLong,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, routeID, err := parseRouteArgs(args)
		if err != nil {
			return err
		}
		f := &tcpRouteUpdateFields
		if err := f.validate(); err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewTCPRoute(
			routeID, f.name, f.port, f.backend, f.backendPort,
			*pidginhost.NewNullableBool(nil),
			"", "", "",
		)
		if f.namespace != "" {
			body.Namespace = pidginhost.PtrString(f.namespace)
		}
		route, _, err := c.KubernetesAPI.KubernetesClustersTcproutesUpdate(cmd.Context(), clusterID, args[1]).
			TCPRoute(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating TCP route", err)
		}
		if route == nil {
			return fmt.Errorf("updating TCP route %d: server returned no route", routeID)
		}
		if route.Id != routeID {
			return fmt.Errorf("updating TCP route %d: server returned route %d", routeID, route.Id)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route,
			"TCP route %d updated (Name: %s).\n", route.Id, route.Name)
	},
}

var udpRouteUpdateCmd = &cobra.Command{
	Use:   "update <cluster-id> <route-id>",
	Short: "Replace a UDP route",
	Long:  "Replace a UDP route.\n\n" + routeUpdateLong,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, routeID, err := parseRouteArgs(args)
		if err != nil {
			return err
		}
		f := &udpRouteUpdateFields
		if err := f.validate(); err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewUDPRoute(
			routeID, f.name, f.port, f.backend, f.backendPort,
			*pidginhost.NewNullableBool(nil),
			"", "", "",
		)
		if f.namespace != "" {
			body.Namespace = pidginhost.PtrString(f.namespace)
		}
		route, _, err := c.KubernetesAPI.KubernetesClustersUdproutesUpdate(cmd.Context(), clusterID, args[1]).
			UDPRoute(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating UDP route", err)
		}
		if route == nil {
			return fmt.Errorf("updating UDP route %d: server returned no route", routeID)
		}
		if route.Id != routeID {
			return fmt.Errorf("updating UDP route %d: server returned route %d", routeID, route.Id)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route,
			"UDP route %d updated (Name: %s).\n", route.Id, route.Name)
	},
}

func parseRouteArgs(args []string) (clusterID, routeID int32, err error) {
	if clusterID, err = cmdutil.ParseInt32(args[0]); err != nil {
		return 0, 0, err
	}
	if routeID, err = cmdutil.ParseInt32(args[1]); err != nil {
		return 0, 0, err
	}
	return clusterID, routeID, nil
}

func init() {
	httpRouteUpdateCmd.Flags().StringVar(&httpRouteUpdateName, "name", "", "Route name (required)")
	httpRouteUpdateCmd.Flags().StringArrayVar(&httpRouteUpdateHostnames, "hostname", nil, "Hostname to route (repeatable, required)")
	httpRouteUpdateCmd.Flags().StringVar(&httpRouteUpdateBackend, "backend", "", "Backend service name (required)")
	httpRouteUpdateCmd.Flags().Int32Var(&httpRouteUpdatePort, "port", 0, "Backend service port (required)")
	httpRouteUpdateCmd.Flags().StringVar(&httpRouteUpdateNamespace, "namespace", "", "Kubernetes namespace")
	httpRouteUpdateCmd.Flags().StringVar(&httpRouteUpdatePrefix, "path-prefix", "", "Path prefix to match")
	httpRouteUpdateCmd.Flags().BoolVar(&httpRouteUpdateTLS, "tls", true, "Enable TLS termination")

	tcpRouteUpdateFields.register(tcpRouteUpdateCmd)
	udpRouteUpdateFields.register(udpRouteUpdateCmd)

	httpRouteCmd.AddCommand(httpRouteGetCmd)
	httpRouteCmd.AddCommand(httpRouteUpdateCmd)
	tcpRouteCmd.AddCommand(tcpRouteGetCmd)
	tcpRouteCmd.AddCommand(tcpRouteUpdateCmd)
	udpRouteCmd.AddCommand(udpRouteGetCmd)
	udpRouteCmd.AddCommand(udpRouteUpdateCmd)
}
