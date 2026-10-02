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

// A port forward publishes a service inside the cluster on the load balancer's
// public IP. What reaches it is then governed by the LB firewall rules, so the
// two commands are usually used together.

var portForwardCmd = &cobra.Command{
	Use:     "port-forward",
	Aliases: []string{"pf"},
	Short:   "Manage load balancer port forwards",
	Args:    cobra.NoArgs,
}

// portForwardFields carries the writable fields. create and update each own an
// instance so "was this flag given?" stays per-command.
type portForwardFields struct {
	owner *cobra.Command

	internalIP string
	port       int32
	protocol   string
}

var portForwardFieldFlags = []string{"internal-ip", "port", "protocol"}

func (f *portForwardFields) register(cmd *cobra.Command, protocolDefault string) {
	f.owner = cmd
	cmd.Flags().StringVar(&f.internalIP, "internal-ip", "", "Address inside the cluster to forward to")
	cmd.Flags().Int32Var(&f.port, "port", 0, "Port to forward")
	cmd.Flags().StringVar(&f.protocol, "protocol", protocolDefault, "Protocol: tcp or udp")
}

func (f *portForwardFields) isSet(name string) bool {
	return f.owner != nil && f.owner.Flags().Changed(name)
}

func (f *portForwardFields) anySet() bool {
	for _, name := range portForwardFieldFlags {
		if f.isSet(name) {
			return true
		}
	}
	return false
}

func (f *portForwardFields) validateChanged() error {
	if f.isSet("internal-ip") {
		if err := validateNonEmptyFlag("internal-ip", f.internalIP); err != nil {
			return err
		}
	}
	if f.isSet("port") {
		if err := validateNetworkPort("port", f.port); err != nil {
			return err
		}
	}
	return nil
}

// protocol is an enum server-side. Validate here so a typo fails before it
// reaches a live load balancer.
func (f *portForwardFields) parsedProtocol() (*pidginhost.ProtocolEnum, error) {
	v, err := pidginhost.NewProtocolEnumFromValue(f.protocol)
	if err != nil {
		return nil, fmt.Errorf("invalid --protocol %q: must be tcp or udp", f.protocol)
	}
	return v, nil
}

var (
	portForwardCreateFields portForwardFields
	portForwardUpdateFields portForwardFields
)

var portForwardListCmd = &cobra.Command{
	Use:   "list <cluster-id>",
	Short: "List load balancer port forwards",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		forwards, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.K8sPortForward, bool, error) {
			resp, _, err := c.KubernetesAPI.KubernetesClustersPortForwardsList(cmd.Context(), clusterID).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			if resp == nil {
				return nil, false, fmt.Errorf("server returned no port forward list")
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return cmdutil.APIError("listing port forwards", err)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), forwards, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "INTERNAL IP", "PORT", "PROTOCOL")
			for _, f := range forwards {
				output.PrintRow(tw, f.Id, f.InternalIp, f.Port, f.Protocol)
			}
			tw.Flush()
		})
	},
}

var portForwardGetCmd = &cobra.Command{
	Use:   "get <cluster-id> <forward-id>",
	Short: "Get a load balancer port forward",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, forwardID, err := parseRouteArgs(args)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		forward, _, err := c.KubernetesAPI.KubernetesClustersPortForwardsRetrieve(cmd.Context(), clusterID, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting port forward", err)
		}
		if forward == nil {
			return fmt.Errorf("getting port forward %d: server returned no port forward", forwardID)
		}
		if forward.Id != forwardID {
			return fmt.Errorf("getting port forward %d: server returned port forward %d", forwardID, forward.Id)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), forward, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", forward.Id)
			output.PrintRow(tw, "Internal IP:", forward.InternalIp)
			output.PrintRow(tw, "Port:", forward.Port)
			output.PrintRow(tw, "Protocol:", forward.Protocol)
			tw.Flush()
		})
	},
}

var portForwardCreateCmd = &cobra.Command{
	Use:   "create <cluster-id>",
	Short: "Create a load balancer port forward",
	Long: "Create a load balancer port forward.\n\n" +
		"This publishes an address inside the cluster on the load balancer's\n" +
		"public IP. Which sources may reach it is governed separately by\n" +
		"'phctl kubernetes lb-firewall'.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		f := &portForwardCreateFields
		if f.internalIP == "" {
			return fmt.Errorf("--internal-ip is required")
		}
		if err := validateNonEmptyFlag("internal-ip", f.internalIP); err != nil {
			return err
		}
		if f.port == 0 {
			return fmt.Errorf("--port is required")
		}
		if err := validateNetworkPort("port", f.port); err != nil {
			return err
		}
		protocol, err := f.parsedProtocol()
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewK8sPortForwardRequest(f.internalIP, f.port, *protocol)
		forward, _, err := c.KubernetesAPI.KubernetesClustersPortForwardsCreate(cmd.Context(), clusterID).
			K8sPortForwardRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("creating port forward", err)
		}
		if forward == nil {
			return fmt.Errorf("creating port forward: server returned no port forward")
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), forward,
			"Port forward created (ID: %d, %s:%d/%s).\n", forward.Id, forward.InternalIp, forward.Port, forward.Protocol)
	},
}

var portForwardUpdateCmd = &cobra.Command{
	Use:   "update <cluster-id> <forward-id>",
	Short: "Update a load balancer port forward",
	Long:  "Update a load balancer port forward.\n\nOnly the fields you pass are sent.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, forwardID, err := parseRouteArgs(args)
		if err != nil {
			return err
		}
		f := &portForwardUpdateFields
		// A PATCH with an empty body answers 200 having changed nothing.
		if !f.anySet() {
			return fmt.Errorf("nothing to update: pass at least one of --%s", joinFlags(portForwardFieldFlags))
		}
		if err := f.validateChanged(); err != nil {
			return err
		}
		var body pidginhost.PatchedK8sPortForwardRequest
		if f.isSet("internal-ip") {
			body.InternalIp = pidginhost.PtrString(f.internalIP)
		}
		if f.isSet("port") {
			body.Port = pidginhost.PtrInt32(f.port)
		}
		if f.isSet("protocol") {
			protocol, err := f.parsedProtocol()
			if err != nil {
				return err
			}
			body.Protocol = protocol
		}

		c, err := newClient()
		if err != nil {
			return err
		}
		forward, _, err := c.KubernetesAPI.KubernetesClustersPortForwardsPartialUpdate(cmd.Context(), clusterID, args[1]).
			PatchedK8sPortForwardRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating port forward", err)
		}
		if forward == nil {
			return fmt.Errorf("updating port forward %d: server returned no port forward", forwardID)
		}
		if forward.Id != forwardID {
			return fmt.Errorf("updating port forward %d: server returned port forward %d", forwardID, forward.Id)
		}
		// The route answers 200 whether or not it applied anything.
		if f.isSet("internal-ip") && forward.InternalIp != f.internalIP {
			return fmt.Errorf("updating port forward %d: asked for internal-ip=%s, it still reports %s",
				forwardID, f.internalIP, forward.InternalIp)
		}
		if f.isSet("port") && forward.Port != f.port {
			return fmt.Errorf("updating port forward %d: asked for port=%d, it still reports %d",
				forwardID, f.port, forward.Port)
		}
		if f.isSet("protocol") && string(forward.Protocol) != f.protocol {
			return fmt.Errorf("updating port forward %d: asked for protocol=%s, it still reports %s",
				forwardID, f.protocol, forward.Protocol)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), forward,
			"Port forward %d updated.\n", forward.Id)
	},
}

var portForwardDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <forward-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a load balancer port forward",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if _, err := cmdutil.ParseInt32(args[1]); err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Delete port forward %s from cluster %d? Whatever reaches the service through it stops.", args[1], clusterID)) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersPortForwardsDestroy(cmd.Context(), clusterID, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("deleting port forward", err)
		}
		cmd.Printf("Port forward %s deleted.\n", args[1])
		return nil
	},
}

func init() {
	// create carries the real default; update sends only what it is given, so a
	// default there would read as "omit this and it resets to tcp".
	portForwardCreateFields.register(portForwardCreateCmd, "tcp")
	portForwardUpdateFields.register(portForwardUpdateCmd, "")

	portForwardCmd.AddCommand(portForwardListCmd)
	portForwardCmd.AddCommand(portForwardGetCmd)
	portForwardCmd.AddCommand(portForwardCreateCmd)
	portForwardCmd.AddCommand(portForwardUpdateCmd)
	portForwardCmd.AddCommand(portForwardDeleteCmd)

	Cmd.AddCommand(portForwardCmd)
}
