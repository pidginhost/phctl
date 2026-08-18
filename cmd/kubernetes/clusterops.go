package kubernetes

import (
	"fmt"
	"io"
	"strings"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

// --- Cluster features ---

var (
	upgradeFeatureName  string
	upgradeFeatureRetry bool
)

var clusterUpgradeFeatureCmd = &cobra.Command{
	Use:   "upgrade-feature <cluster-id>",
	Short: "Upgrade a cluster feature to the latest compatible version",
	Long: "Upgrade a cluster feature (cert-manager, ingress, and so on) to the\n" +
		"latest version compatible with the cluster's Kubernetes release.\n\n" +
		"Use --retry to re-run an install that previously failed.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if upgradeFeatureName == "" {
			return fmt.Errorf("--feature is required")
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Upgrade feature %q on cluster %s? Its workloads restart.", upgradeFeatureName, args[0])) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewFeatureUpgradeRequest(upgradeFeatureName)
		if upgradeFeatureRetry {
			body.Retry = pidginhost.PtrBool(true)
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersUpgradeFeatureCreate(cmd.Context(), args[0]).
			FeatureUpgradeRequest(body).Execute()
		if err != nil {
			return cmdutil.APIError("upgrading feature", err)
		}
		if resp == nil {
			return fmt.Errorf("upgrading feature %q: server returned no result", upgradeFeatureName)
		}
		// The route answers 200 whether or not it queued anything.
		if resp.Status != "OK" {
			return fmt.Errorf("upgrading feature %q: server reported status %q: %s",
				upgradeFeatureName, resp.Status, resp.Message)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"%s\n", resp.Message)
	},
}

// --- Cluster settings ---

var (
	clusterUpdateName      string
	clusterUpdateProtected bool
	clusterUpdateFeatures  []string
	// Captured in init: a RunE closure cannot refer to the command that owns
	// it without creating an initialization cycle.
	clusterUpdateFlagSet *pflag.FlagSet
)

var clusterUpdateCmd = &cobra.Command{
	Use:   "update <cluster-id>",
	Short: "Change a cluster's name, delete protection or features",
	Long: "Change a cluster's settings.\n\n" +
		"Only the flags you pass are sent. --features replaces the whole feature\n" +
		"set rather than adding to it, and installing or removing a feature\n" +
		"restarts the workloads behind it.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		nameSet := clusterUpdateFlagSet.Changed("name")
		protectedSet := clusterUpdateFlagSet.Changed("protected")
		featuresSet := clusterUpdateFlagSet.Changed("features")
		if !nameSet && !protectedSet && !featuresSet {
			return fmt.Errorf("nothing to update: pass at least one of --name, --protected or --features")
		}

		body := *pidginhost.NewPatchedClusterDetail()
		if nameSet {
			body.Name = pidginhost.PtrString(clusterUpdateName)
		}
		if protectedSet {
			body.Protected = pidginhost.PtrBool(clusterUpdateProtected)
		}

		var wantFeatures []pidginhost.FeaturesEnum
		if featuresSet {
			// The generated model tags features with omitempty, so an empty
			// list is dropped from the request and the server would answer 200
			// having changed nothing.
			if len(clusterUpdateFeatures) == 0 {
				return fmt.Errorf("removing every feature is not supported by the API client; " +
					"pass the features the cluster should keep instead")
			}
			for _, name := range clusterUpdateFeatures {
				feature, err := pidginhost.NewFeaturesEnumFromValue(name)
				if err != nil {
					return fmt.Errorf("invalid --features entry %q: must be one of %s",
						name, featureNames())
				}
				wantFeatures = append(wantFeatures, *feature)
			}
			body.Features = wantFeatures
		}

		if featuresSet && !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Set cluster %d features to %v? Features it no longer lists are removed and their workloads restart.",
				clusterID, clusterUpdateFeatures)) {
			return nil
		}

		c, err := newClient()
		if err != nil {
			return err
		}
		cluster, _, err := c.KubernetesAPI.KubernetesClustersPartialUpdate(cmd.Context(), args[0]).
			PatchedClusterDetail(body).Execute()
		if err != nil {
			return cmdutil.APIError("updating cluster", err)
		}
		if cluster == nil {
			return fmt.Errorf("updating cluster %d: server returned no cluster", clusterID)
		}
		if cluster.Id != clusterID {
			return fmt.Errorf("updating cluster %d: server returned cluster %d", clusterID, cluster.Id)
		}
		// The route answers 200 whether or not it applied the change, so read
		// the settings back out of the response it returned.
		if nameSet && output.Pstr(cluster.Name) != clusterUpdateName {
			return fmt.Errorf("updating cluster %d: asked for name %q, cluster still reports %s",
				clusterID, clusterUpdateName, output.Pstr(cluster.Name))
		}
		if protectedSet && (cluster.Protected == nil || *cluster.Protected != clusterUpdateProtected) {
			return fmt.Errorf("updating cluster %d: asked for protected=%t, cluster still reports %s",
				clusterID, clusterUpdateProtected, output.Pstr(cluster.Protected))
		}
		if featuresSet && !sameFeatures(cluster.Features, wantFeatures) {
			return fmt.Errorf("updating cluster %d: asked for features %v, cluster still reports %v",
				clusterID, wantFeatures, cluster.Features)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), cluster,
			"Cluster %d updated.\n", cluster.Id)
	},
}

// sameFeatures compares two feature sets ignoring order.
func sameFeatures(got, want []pidginhost.FeaturesEnum) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[pidginhost.FeaturesEnum]int, len(got))
	for _, f := range got {
		seen[f]++
	}
	for _, f := range want {
		if seen[f] == 0 {
			return false
		}
		seen[f]--
	}
	return true
}

func featureNames() string {
	names := make([]string, 0, len(pidginhost.AllowedFeaturesEnumEnumValues))
	for _, f := range pidginhost.AllowedFeaturesEnumEnumValues {
		names = append(names, string(f))
	}
	return strings.Join(names, ", ")
}

// --- Cloud VM access ---

var clusterEligibleVMsCmd = &cobra.Command{
	Use:   "eligible-vms <cluster-id>",
	Short: "List cloud VMs that can be connected to the cluster",
	Long: "List cloud VMs eligible for connection to the cluster's private network.\n\n" +
		"The list is empty while cloud VM access is disabled -- enable it with\n" +
		"'phctl kubernetes cluster toggle-vm-access'.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersEligibleVmsRetrieve(cmd.Context(), args[0]).Execute()
		if err != nil {
			return cmdutil.APIError("listing eligible VMs", err)
		}
		if resp == nil {
			return fmt.Errorf("listing eligible VMs for cluster %s: server returned no result", args[0])
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp.Vms, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME")
			for _, vm := range resp.Vms {
				output.PrintRow(tw, vm.Id, vm.Hostname)
			}
			tw.Flush()
		})
	},
}

var clusterToggleVMAccessCmd = &cobra.Command{
	Use:   "toggle-vm-access <cluster-id>",
	Short: "Turn cloud VM access to the cluster network on or off",
	Long: "Turn cloud VM access on or off for a cluster.\n\n" +
		"This flips the current setting rather than setting it, so running it\n" +
		"twice returns to where you started. The response reports the state the\n" +
		"cluster ended up in.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Flip cloud VM access on cluster %s?", args[0])) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersToggleCloudVmAccessCreate(cmd.Context(), args[0]).Execute()
		if err != nil {
			return cmdutil.APIError("toggling cloud VM access", err)
		}
		if resp == nil {
			return fmt.Errorf("toggling cloud VM access on cluster %s: server returned no result", args[0])
		}
		state := "disabled"
		if resp.Enabled {
			state = "enabled"
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp,
			"Cloud VM access is now %s on cluster %s.\n", state, args[0])
	},
}

// --- Resource pools ---

var poolGetCmd = &cobra.Command{
	Use:   "get <cluster-id> <pool-id>",
	Short: "Get resource pool details",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		poolID, err := cmdutil.ParseInt32(args[1])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		pool, _, err := c.KubernetesAPI.KubernetesClustersResourcePoolsRetrieve(cmd.Context(), clusterID, args[1]).Execute()
		if err != nil {
			return cmdutil.APIError("getting pool", err)
		}
		if pool == nil {
			return fmt.Errorf("getting pool %d: server returned no pool", poolID)
		}
		if pool.Id != poolID {
			return fmt.Errorf("getting pool %d: server returned pool %d", poolID, pool.Id)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), pool, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", pool.Id)
			output.PrintRow(tw, "Package:", pool.Package)
			output.PrintRow(tw, "Generation:", pool.Generation)
			output.PrintRow(tw, "Size:", pool.Size)
			tw.Flush()
			if len(pool.Nodes) == 0 {
				return
			}
			fmt.Fprintln(w)
			nodes := output.NewTabWriter(w)
			output.PrintRow(nodes, "NODE ID", "NAME", "IP")
			for _, n := range pool.Nodes {
				output.PrintRow(nodes, n.Id, n.Name, n.Ip)
			}
			nodes.Flush()
		})
	},
}

var poolResizeSize int32

var poolResizeCmd = &cobra.Command{
	Use:   "resize <cluster-id> <pool-id>",
	Short: "Change the number of nodes in a resource pool",
	Long: "Change the number of nodes in a resource pool.\n\n" +
		"Growing a pool provisions billable VMs and shrinking one destroys them.\n" +
		"The work runs asynchronously, so the pool still reports its previous size\n" +
		"when the command returns; follow it with 'phctl kubernetes pool get'.",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		poolID, err := cmdutil.ParseInt32(args[1])
		if err != nil {
			return err
		}
		// The API treats a body without new_size as a no-op and still answers
		// 200, so refuse to send one.
		if poolResizeSize < 1 {
			return fmt.Errorf("--size must be at least 1, got %d", poolResizeSize)
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(),
			fmt.Sprintf("Resize pool %d on cluster %d to %d node(s)? This changes what it costs.",
				poolID, clusterID, poolResizeSize)) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewPatchedResourcePool()
		body.NewSize = pidginhost.PtrInt32(poolResizeSize)
		pool, _, err := c.KubernetesAPI.KubernetesClustersResourcePoolsPartialUpdate(cmd.Context(), clusterID, args[1]).
			PatchedResourcePool(body).Execute()
		if err != nil {
			return cmdutil.APIError("resizing pool", err)
		}
		if pool == nil {
			return fmt.Errorf("resizing pool %d: server returned no pool", poolID)
		}
		if pool.Id != poolID {
			return fmt.Errorf("resizing pool %d: server returned pool %d", poolID, pool.Id)
		}
		return output.Result(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), pool,
			"Resize of pool %d to %d node(s) requested; it currently reports %s.\n",
			pool.Id, poolResizeSize, pool.Size)
	},
}

// --- Pool nodes ---

var nodeGetCmd = &cobra.Command{
	Use:   "get <cluster-id> <pool-id> <node-id>",
	Short: "Get node details",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, poolID, nodeID, err := parseNodeArgs(args)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		node, _, err := c.KubernetesAPI.KubernetesClustersResourcePoolsNodesRetrieve(cmd.Context(), clusterID, args[2], poolID).Execute()
		if err != nil {
			return cmdutil.APIError("getting node", err)
		}
		if node == nil {
			return fmt.Errorf("getting node %d: server returned no node", nodeID)
		}
		if node.Id != nodeID {
			return fmt.Errorf("getting node %d: server returned node %d", nodeID, node.Id)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), node, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", node.Id)
			output.PrintRow(tw, "Name:", node.Name)
			output.PrintRow(tw, "IP:", node.Ip)
			tw.Flush()
		})
	},
}

var nodeRRDCmd = &cobra.Command{
	Use:   "rrd <cluster-id> <pool-id> <node-id>",
	Short: "Show historical metrics for a node",
	Long: "Show the node's recorded metric series.\n\n" +
		"The API accepts a timeframe but does not document it, so the SDK cannot\n" +
		"send one and this command always returns the server's default window.",
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterID, poolID, nodeID, err := parseNodeArgs(args)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersResourcePoolsNodesRrdRetrieve(cmd.Context(), clusterID, args[2], poolID).Execute()
		if err != nil {
			return cmdutil.APIError("getting node RRD data", err)
		}
		if resp == nil {
			return fmt.Errorf("getting RRD data for node %d: server returned no data", nodeID)
		}
		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp, func(w io.Writer) {
			fmt.Fprintf(w, "Timeframe: %s\n\n", resp.Timeframe)
			printRRDPoints(w, resp.Data)
		})
	},
}

// printRRDPoints renders the series. The schema types "data" as an untyped
// array, so a point is only rendered column-wise when it really is an object.
func printRRDPoints(w io.Writer, data []interface{}) {
	tw := output.NewTabWriter(w)
	output.PrintRow(tw, "TIME", "CPU", "MEM", "MAXMEM", "NETIN", "NETOUT")
	for _, raw := range data {
		point, ok := raw.(map[string]interface{})
		if !ok {
			output.PrintRow(tw, fmt.Sprintf("%v", raw))
			continue
		}
		output.PrintRow(tw,
			rrdField(point, "time"), rrdField(point, "cpu"), rrdField(point, "mem"),
			rrdField(point, "maxmem"), rrdField(point, "netin"), rrdField(point, "netout"))
	}
	tw.Flush()
}

func rrdField(point map[string]interface{}, key string) string {
	v, ok := point[key]
	if !ok || v == nil {
		return "<none>"
	}
	return fmt.Sprintf("%v", v)
}

func parseNodeArgs(args []string) (clusterID, poolID, nodeID int32, err error) {
	if clusterID, err = cmdutil.ParseInt32(args[0]); err != nil {
		return 0, 0, 0, err
	}
	if poolID, err = cmdutil.ParseInt32(args[1]); err != nil {
		return 0, 0, 0, err
	}
	if nodeID, err = cmdutil.ParseInt32(args[2]); err != nil {
		return 0, 0, 0, err
	}
	return clusterID, poolID, nodeID, nil
}

func init() {
	clusterUpgradeFeatureCmd.Flags().StringVar(&upgradeFeatureName, "feature", "", "Feature to upgrade, e.g. cert-manager (required)")
	clusterUpgradeFeatureCmd.Flags().BoolVar(&upgradeFeatureRetry, "retry", false, "Re-run a failed install")
	if err := clusterUpgradeFeatureCmd.MarkFlagRequired("feature"); err != nil {
		panic(err)
	}

	poolResizeCmd.Flags().Int32Var(&poolResizeSize, "size", 0, "Number of nodes the pool should have (required)")
	if err := poolResizeCmd.MarkFlagRequired("size"); err != nil {
		panic(err)
	}

	clusterUpdateCmd.Flags().StringVar(&clusterUpdateName, "name", "", "New cluster name")
	clusterUpdateCmd.Flags().BoolVar(&clusterUpdateProtected, "protected", false, "Protect the cluster from deletion")
	clusterUpdateCmd.Flags().StringSliceVar(&clusterUpdateFeatures, "features", nil,
		"Comma-separated feature set the cluster should have; replaces the current set")

	clusterUpdateFlagSet = clusterUpdateCmd.Flags()

	clusterCmd.AddCommand(clusterUpdateCmd)
	clusterCmd.AddCommand(clusterUpgradeFeatureCmd)
	clusterCmd.AddCommand(clusterEligibleVMsCmd)
	clusterCmd.AddCommand(clusterToggleVMAccessCmd)

	poolCmd.AddCommand(poolGetCmd)
	poolCmd.AddCommand(poolResizeCmd)

	nodeCmd.AddCommand(nodeGetCmd)
	nodeCmd.AddCommand(nodeRRDCmd)
}
