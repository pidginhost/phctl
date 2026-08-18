package kubernetes

import (
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/output"
)

// Gateway route detail (get) and update.
//
// These reads had no response schema until the API declared one, so get and
// list decoded the body by hand and update had to be a PUT that replaced the
// whole route. Both now use the generated models, and update is a PATCH that
// sends only the fields the caller passed.

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
		route, _, err := c.KubernetesAPI.KubernetesClustersHttproutesRetrieve(cmd.Context(), clusterID, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting HTTP route", err)
		}
		if route == nil {
			return fmt.Errorf("getting HTTP route %d: server returned no route", routeID)
		}
		if route.Id != routeID {
			return fmt.Errorf("getting HTTP route %d: server returned route %d", routeID, route.Id)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route, func(w io.Writer) {
			printHTTPRoute(w, route)
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
		route, _, err := c.KubernetesAPI.KubernetesClustersTcproutesRetrieve(cmd.Context(), clusterID, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting TCP route", err)
		}
		if route == nil {
			return fmt.Errorf("getting TCP route %d: server returned no route", routeID)
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
		route, _, err := c.KubernetesAPI.KubernetesClustersUdproutesRetrieve(cmd.Context(), clusterID, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting UDP route", err)
		}
		if route == nil {
			return fmt.Errorf("getting UDP route %d: server returned no route", routeID)
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

// --- update ---
//
// PATCH, not PUT: the server rebuilds the route from its current field values
// merged with the change, so omitting a field keeps it. Sending a full body
// would silently reset anything the caller did not restate.

const routeUpdateLong = "Only the fields you pass are sent; everything else keeps its current value.\n" +
	"Updating re-applies the route to the cluster."

// httpRouteFields holds the writable fields of an HTTP route for update.
type httpRouteFields struct {
	owner *cobra.Command

	name      string
	hostnames []string
	backend   string
	port      int32
	namespace string
	backendNS string
	prefix    string
	tls       bool
}

var httpRouteUpdateFieldFlags = []string{
	"name", "hostname", "backend", "port", "namespace", "backend-namespace", "path-prefix", "tls",
}

func (f *httpRouteFields) register(cmd *cobra.Command) {
	f.owner = cmd
	cmd.Flags().StringVar(&f.name, "name", "", "Route name")
	cmd.Flags().StringArrayVar(&f.hostnames, "hostname", nil, "Hostname to route (repeatable; replaces the current list)")
	cmd.Flags().StringVar(&f.backend, "backend", "", "Backend service name")
	cmd.Flags().Int32Var(&f.port, "port", 0, "Backend service port")
	cmd.Flags().StringVar(&f.namespace, "namespace", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&f.backendNS, "backend-namespace", "", "Backend service namespace")
	cmd.Flags().StringVar(&f.prefix, "path-prefix", "", "Path prefix to match")
	cmd.Flags().BoolVar(&f.tls, "tls", true, "Enable TLS termination")
}

func (f *httpRouteFields) isSet(name string) bool {
	return f.owner != nil && f.owner.Flags().Changed(name)
}

func (f *httpRouteFields) anySet() bool {
	for _, name := range httpRouteUpdateFieldFlags {
		if f.isSet(name) {
			return true
		}
	}
	return false
}

var httpRouteUpdateFields httpRouteFields

var httpRouteUpdateCmd = &cobra.Command{
	Use:   "update <cluster-id> <route-id>",
	Short: "Update an HTTP route",
	Long:  "Update an HTTP route.\n\n" + routeUpdateLong,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, routeID, err := parseRouteArgs(args)
		if err != nil {
			return err
		}
		f := &httpRouteUpdateFields
		if !f.anySet() {
			return fmt.Errorf("nothing to update: pass at least one of --%s", joinFlags(httpRouteUpdateFieldFlags))
		}
		// Not NewPatchedHTTPRoute(): that constructor seeds the schema defaults for
		// backend_namespace, path_prefix and enable_tls, which a PATCH would then
		// send as if the caller had asked for them.
		var body pidginhost.PatchedHTTPRoute
		if f.isSet("name") {
			body.Name = pidginhost.PtrString(f.name)
		}
		if f.isSet("hostname") {
			body.Hostnames = f.hostnames
		}
		if f.isSet("backend") {
			body.BackendServiceName = pidginhost.PtrString(f.backend)
		}
		if f.isSet("port") {
			body.BackendServicePort = pidginhost.PtrInt32(f.port)
		}
		if f.isSet("namespace") {
			body.Namespace = pidginhost.PtrString(f.namespace)
		}
		if f.isSet("backend-namespace") {
			body.BackendNamespace = pidginhost.PtrString(f.backendNS)
		}
		if f.isSet("path-prefix") {
			body.PathPrefix = pidginhost.PtrString(f.prefix)
		}
		if f.isSet("tls") {
			body.EnableTls = pidginhost.PtrBool(f.tls)
		}

		c, err := newClient()
		if err != nil {
			return err
		}
		route, _, err := c.KubernetesAPI.KubernetesClustersHttproutesPartialUpdate(cmd.Context(), clusterID, args[1]).
			PatchedHTTPRoute(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating HTTP route", err)
		}
		if route == nil {
			return fmt.Errorf("updating HTTP route %d: server returned no route", routeID)
		}
		if route.Id != routeID {
			return fmt.Errorf("updating HTTP route %d: server returned route %d", routeID, route.Id)
		}
		if err := f.verifyApplied(routeID, route); err != nil {
			return err
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route,
			"HTTP route %d updated (Name: %s).\n", route.Id, route.Name)
	},
}

// The route answers 200 whether or not it applied anything, so read each field
// the caller asked for back out of the response.
func (f *httpRouteFields) verifyApplied(routeID int32, route *pidginhost.HTTPRoute) error {
	mismatch := func(field string, want, got any) error {
		return fmt.Errorf("updating HTTP route %d: asked for %s=%v, route still reports %v", routeID, field, want, got)
	}
	if f.isSet("name") && route.Name != f.name {
		return mismatch("name", f.name, route.Name)
	}
	if f.isSet("hostname") && !sameStrings(route.Hostnames, f.hostnames) {
		return mismatch("hostname", f.hostnames, route.Hostnames)
	}
	if f.isSet("backend") && route.BackendServiceName != f.backend {
		return mismatch("backend", f.backend, route.BackendServiceName)
	}
	if f.isSet("port") && route.BackendServicePort != f.port {
		return mismatch("port", f.port, route.BackendServicePort)
	}
	if f.isSet("namespace") && output.Pstr(route.Namespace) != f.namespace {
		return mismatch("namespace", f.namespace, output.Pstr(route.Namespace))
	}
	if f.isSet("backend-namespace") && output.Pstr(route.BackendNamespace) != f.backendNS {
		return mismatch("backend-namespace", f.backendNS, output.Pstr(route.BackendNamespace))
	}
	if f.isSet("path-prefix") && output.Pstr(route.PathPrefix) != f.prefix {
		return mismatch("path-prefix", f.prefix, output.Pstr(route.PathPrefix))
	}
	if f.isSet("tls") && (route.EnableTls == nil || *route.EnableTls != f.tls) {
		return mismatch("tls", f.tls, output.Pstr(route.EnableTls))
	}
	return nil
}

// portRouteFields are the writable fields TCP and UDP routes share.
type portRouteFields struct {
	owner *cobra.Command

	name        string
	port        int32
	backend     string
	backendPort int32
	namespace   string
	backendNS   string
}

var portRouteUpdateFieldFlags = []string{"name", "port", "backend", "backend-port", "namespace", "backend-namespace"}

func (f *portRouteFields) register(cmd *cobra.Command) {
	f.owner = cmd
	cmd.Flags().StringVar(&f.name, "name", "", "Route name")
	cmd.Flags().Int32Var(&f.port, "port", 0, "External port to expose")
	cmd.Flags().StringVar(&f.backend, "backend", "", "Backend service name")
	cmd.Flags().Int32Var(&f.backendPort, "backend-port", 0, "Backend service port")
	cmd.Flags().StringVar(&f.namespace, "namespace", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&f.backendNS, "backend-namespace", "", "Backend service namespace")
}

func (f *portRouteFields) isSet(name string) bool {
	return f.owner != nil && f.owner.Flags().Changed(name)
}

func (f *portRouteFields) anySet() bool {
	for _, name := range portRouteUpdateFieldFlags {
		if f.isSet(name) {
			return true
		}
	}
	return false
}

// verifyApplied reads back the fields the caller asked to change.
func (f *portRouteFields) verifyApplied(kind string, routeID int32, name string, namespace *string,
	port int32, backend string, backendPort int32, backendNS *string) error {
	mismatch := func(field string, want, got any) error {
		return fmt.Errorf("updating %s route %d: asked for %s=%v, route still reports %v",
			kind, routeID, field, want, got)
	}
	if f.isSet("name") && name != f.name {
		return mismatch("name", f.name, name)
	}
	if f.isSet("port") && port != f.port {
		return mismatch("port", f.port, port)
	}
	if f.isSet("backend") && backend != f.backend {
		return mismatch("backend", f.backend, backend)
	}
	if f.isSet("backend-port") && backendPort != f.backendPort {
		return mismatch("backend-port", f.backendPort, backendPort)
	}
	if f.isSet("namespace") && output.Pstr(namespace) != f.namespace {
		return mismatch("namespace", f.namespace, output.Pstr(namespace))
	}
	if f.isSet("backend-namespace") && output.Pstr(backendNS) != f.backendNS {
		return mismatch("backend-namespace", f.backendNS, output.Pstr(backendNS))
	}
	return nil
}

var (
	tcpRouteUpdateFields portRouteFields
	udpRouteUpdateFields portRouteFields
)

var tcpRouteUpdateCmd = &cobra.Command{
	Use:   "update <cluster-id> <route-id>",
	Short: "Update a TCP route",
	Long:  "Update a TCP route.\n\n" + routeUpdateLong,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, routeID, err := parseRouteArgs(args)
		if err != nil {
			return err
		}
		f := &tcpRouteUpdateFields
		if !f.anySet() {
			return fmt.Errorf("nothing to update: pass at least one of --%s", joinFlags(portRouteUpdateFieldFlags))
		}
		// Zero value, not the constructor: it seeds backend_namespace.
		var body pidginhost.PatchedTCPRoute
		if f.isSet("name") {
			body.Name = pidginhost.PtrString(f.name)
		}
		if f.isSet("port") {
			body.Port = pidginhost.PtrInt32(f.port)
		}
		if f.isSet("backend") {
			body.BackendServiceName = pidginhost.PtrString(f.backend)
		}
		if f.isSet("backend-port") {
			body.BackendServicePort = pidginhost.PtrInt32(f.backendPort)
		}
		if f.isSet("namespace") {
			body.Namespace = pidginhost.PtrString(f.namespace)
		}
		if f.isSet("backend-namespace") {
			body.BackendNamespace = pidginhost.PtrString(f.backendNS)
		}

		c, err := newClient()
		if err != nil {
			return err
		}
		route, _, err := c.KubernetesAPI.KubernetesClustersTcproutesPartialUpdate(cmd.Context(), clusterID, args[1]).
			PatchedTCPRoute(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating TCP route", err)
		}
		if route == nil {
			return fmt.Errorf("updating TCP route %d: server returned no route", routeID)
		}
		if route.Id != routeID {
			return fmt.Errorf("updating TCP route %d: server returned route %d", routeID, route.Id)
		}
		if err := f.verifyApplied("TCP", routeID, route.Name, route.Namespace, route.Port,
			route.BackendServiceName, route.BackendServicePort, route.BackendNamespace); err != nil {
			return err
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route,
			"TCP route %d updated (Name: %s).\n", route.Id, route.Name)
	},
}

var udpRouteUpdateCmd = &cobra.Command{
	Use:   "update <cluster-id> <route-id>",
	Short: "Update a UDP route",
	Long:  "Update a UDP route.\n\n" + routeUpdateLong,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, routeID, err := parseRouteArgs(args)
		if err != nil {
			return err
		}
		f := &udpRouteUpdateFields
		if !f.anySet() {
			return fmt.Errorf("nothing to update: pass at least one of --%s", joinFlags(portRouteUpdateFieldFlags))
		}
		// Zero value, not the constructor: it seeds backend_namespace.
		var body pidginhost.PatchedUDPRoute
		if f.isSet("name") {
			body.Name = pidginhost.PtrString(f.name)
		}
		if f.isSet("port") {
			body.Port = pidginhost.PtrInt32(f.port)
		}
		if f.isSet("backend") {
			body.BackendServiceName = pidginhost.PtrString(f.backend)
		}
		if f.isSet("backend-port") {
			body.BackendServicePort = pidginhost.PtrInt32(f.backendPort)
		}
		if f.isSet("namespace") {
			body.Namespace = pidginhost.PtrString(f.namespace)
		}
		if f.isSet("backend-namespace") {
			body.BackendNamespace = pidginhost.PtrString(f.backendNS)
		}

		c, err := newClient()
		if err != nil {
			return err
		}
		route, _, err := c.KubernetesAPI.KubernetesClustersUdproutesPartialUpdate(cmd.Context(), clusterID, args[1]).
			PatchedUDPRoute(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating UDP route", err)
		}
		if route == nil {
			return fmt.Errorf("updating UDP route %d: server returned no route", routeID)
		}
		if route.Id != routeID {
			return fmt.Errorf("updating UDP route %d: server returned route %d", routeID, route.Id)
		}
		if err := f.verifyApplied("UDP", routeID, route.Name, route.Namespace, route.Port,
			route.BackendServiceName, route.BackendServicePort, route.BackendNamespace); err != nil {
			return err
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), route,
			"UDP route %d updated (Name: %s).\n", route.Id, route.Name)
	},
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
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
	httpRouteUpdateFields.register(httpRouteUpdateCmd)
	tcpRouteUpdateFields.register(tcpRouteUpdateCmd)
	udpRouteUpdateFields.register(udpRouteUpdateCmd)

	httpRouteCmd.AddCommand(httpRouteGetCmd)
	httpRouteCmd.AddCommand(httpRouteUpdateCmd)
	tcpRouteCmd.AddCommand(tcpRouteGetCmd)
	tcpRouteCmd.AddCommand(tcpRouteUpdateCmd)
	udpRouteCmd.AddCommand(udpRouteGetCmd)
	udpRouteCmd.AddCommand(udpRouteUpdateCmd)
}
