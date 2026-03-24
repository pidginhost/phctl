package kubernetes

import (
	"fmt"
	"io"
	"time"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var Cmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"k8s"},
	Short:   "Manage Kubernetes clusters",
	Args:    cobra.NoArgs,
}

// --- Clusters ---

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage Kubernetes clusters",
	Args:  cobra.NoArgs,
}

var clusterListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all clusters",
	RunE: func(cmd *cobra.Command, args []string) error {
		clusters, err := client.RawFetchAll[client.RawCluster](cmd.Context(), "/api/kubernetes/clusters/")
		if err != nil {
			return fmt.Errorf("listing clusters: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, clusters, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "STATUS", "TYPE", "KUBE VERSION", "IPV4")
			for _, cl := range clusters {
				output.PrintRow(tw, cl.Id, output.Pstr(cl.Name), cl.Status, cl.ClusterType, cl.KubeVersion, cl.Ipv4Address)
			}
			tw.Flush()
		})
	},
}

var clusterGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get cluster details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var cl client.RawCluster
		if err := client.RawGet(cmd.Context(), fmt.Sprintf("/api/kubernetes/clusters/%s/", args[0]), &cl); err != nil {
			return fmt.Errorf("getting cluster: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, cl, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", cl.Id)
			output.PrintRow(tw, "Name:", output.Pstr(cl.Name))
			output.PrintRow(tw, "Status:", cl.Status)
			output.PrintRow(tw, "Type:", cl.ClusterType)
			output.PrintRow(tw, "Kube Version:", cl.KubeVersion)
			output.PrintRow(tw, "Talos Version:", cl.TalosVersion)
			output.PrintRow(tw, "IPv4:", cl.Ipv4Address)
			output.PrintRow(tw, "Price/Month:", cl.PricePerMonth)
			output.PrintRow(tw, "Features Ready:", cl.FeaturesReady)
			tw.Flush()
		})
	},
}

var (
	clusterCreateName    string
	clusterCreateType    string
	clusterCreatePkg     string
	clusterCreateSize    int32
	clusterCreateVersion string
	clusterCreateWait    bool
	clusterCreateTimeout time.Duration
)

var clusterCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a Kubernetes cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewClusterAdd(
			pidginhost.ClusterTypeEnum(clusterCreateType),
			clusterCreatePkg,
		)
		if clusterCreateName != "" {
			body.Name = pidginhost.PtrString(clusterCreateName)
		}
		if clusterCreateSize > 0 {
			body.ResourcePoolSize = pidginhost.PtrInt32(clusterCreateSize)
		}
		if clusterCreateVersion != "" {
			v := pidginhost.KubeVersionEnum(clusterCreateVersion)
			body.KubeVersion = &v
		}

		resp, _, err := c.KubernetesAPI.KubernetesClustersCreate(cmd.Context()).ClusterAdd(body).Execute()
		if err != nil {
			return fmt.Errorf("creating cluster: %w", err)
		}
		cmd.Printf("Cluster created (ID: %d)\n", resp.Id)

		if clusterCreateWait {
			id := fmt.Sprintf("%d", resp.Id)
			if err := waitForCluster(cmd.Context(), id, clusterCreateTimeout, cmd); err != nil {
				return err
			}
			cmd.Printf("Cluster %s is active.\n", id)
		}
		return nil
	},
}

var clusterDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a cluster",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := cmdutil.ParseInt32(args[0]); err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete cluster %s?", args[0])) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersDestroy(cmd.Context(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("deleting cluster: %w", err)
		}
		cmd.Printf("Cluster %s deleted.\n", args[0])
		return nil
	},
}

var kubeconfigMerge bool

var clusterKubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig <id>",
	Short: "Get cluster kubeconfig",
	Long: `Download the kubeconfig for a cluster.

By default, prints the raw kubeconfig YAML to stdout so you can redirect it:
  phctl k8s cluster kubeconfig 42 > ~/.kube/my-cluster.yaml

With --merge, the kubeconfig is merged into your existing kubeconfig file
(~/.kube/config or $KUBECONFIG) and the new context is set as current:
  phctl k8s cluster kubeconfig 42 --merge`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersKubeconfigRetrieve(cmd.Context(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("getting kubeconfig: %w", err)
		}

		if kubeconfigMerge {
			path, err := mergeKubeconfig(resp)
			if err != nil {
				return fmt.Errorf("merging kubeconfig: %w", err)
			}
			cmd.Printf("Kubeconfig merged into %s and context set.\n", path)
			return nil
		}

		return output.Print(cmd.OutOrStdout(), cmdutil.OutputFormat(cmd), resp, func(w io.Writer) {
			fmt.Fprintln(w, resp)
		})
	},
}

var (
	upgradeKubeWait    bool
	upgradeKubeTimeout time.Duration
)

var clusterUpgradeKubeCmd = &cobra.Command{
	Use:   "upgrade-kube <id>",
	Short: "Upgrade Kubernetes to the next available version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Upgrade Kubernetes version for cluster %s?", args[0])) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersKubeVersionUpgradeCreate(cmd.Context(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("upgrading kube version: %w", err)
		}
		cmd.Printf("Kubernetes upgrade initiated: %s\n", resp.Status)

		if upgradeKubeWait {
			if err := waitForCluster(cmd.Context(), args[0], upgradeKubeTimeout, cmd); err != nil {
				return err
			}
			cmd.Printf("Cluster %s upgrade complete.\n", args[0])
		}
		return nil
	},
}

var (
	upgradeTalosWait    bool
	upgradeTalosTimeout time.Duration
)

var clusterUpgradeTalosCmd = &cobra.Command{
	Use:   "upgrade-talos <id>",
	Short: "Upgrade Talos to the next available version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Upgrade Talos version for cluster %s?", args[0])) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersTalosVersionUpgradeCreate(cmd.Context(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("upgrading talos version: %w", err)
		}
		cmd.Printf("Talos upgrade initiated: %s\n", resp.Status)

		if upgradeTalosWait {
			if err := waitForCluster(cmd.Context(), args[0], upgradeTalosTimeout, cmd); err != nil {
				return err
			}
			cmd.Printf("Cluster %s upgrade complete.\n", args[0])
		}
		return nil
	},
}

// --- VM connectivity ---

var connectVMServer int32

var clusterConnectVMCmd = &cobra.Command{
	Use:   "connect-vm <cluster-id>",
	Short: "Connect a cloud VM to the cluster private network",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewConnectVMRequest(connectVMServer)
		resp, _, err := c.KubernetesAPI.KubernetesClustersConnectVmCreate(cmd.Context(), args[0]).ConnectVMRequest(body).Execute()
		if err != nil {
			return fmt.Errorf("connecting VM: %w", err)
		}
		cmd.Printf("VM connected: %s - %s\n", resp.Status, resp.Message)
		return nil
	},
}

var disconnectVMServer int32

var clusterDisconnectVMCmd = &cobra.Command{
	Use:   "disconnect-vm <cluster-id>",
	Short: "Disconnect a cloud VM from the cluster private network",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		body := *pidginhost.NewDisconnectVMRequest(disconnectVMServer)
		resp, _, err := c.KubernetesAPI.KubernetesClustersDisconnectVmCreate(cmd.Context(), args[0]).DisconnectVMRequest(body).Execute()
		if err != nil {
			return fmt.Errorf("disconnecting VM: %w", err)
		}
		cmd.Printf("VM disconnected: %s - %s\n", resp.Status, resp.Message)
		return nil
	},
}

var clusterConnectedVMsCmd = &cobra.Command{
	Use:   "connected-vms <cluster-id>",
	Short: "List VMs connected to the cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersConnectedVmsRetrieve(cmd.Context(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("listing connected VMs: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, resp.Vms, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME", "IP")
			for _, vm := range resp.Vms {
				output.PrintRow(tw, vm.Id, vm.Hostname, vm.Ip)
			}
			tw.Flush()
		})
	},
}

// --- Cluster Types ---

var clusterTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List available cluster types",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		types, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.ClusterType, bool, error) {
			resp, _, err := c.KubernetesAPI.KubernetesClusterTypesList(cmd.Context()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing cluster types: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, types, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "TYPE", "MIN WORKERS", "MAX WORKERS", "PACKAGES")
			for _, t := range types {
				output.PrintRow(tw, t.Type, output.Pstr(t.WorkerNodesCountMin), output.Pstr(t.WorkerNodesCountMax), len(t.WorkerNodePackages))
			}
			tw.Flush()
		})
	},
}

// --- Resource Pools ---

var poolCmd = &cobra.Command{
	Use:   "pool",
	Short: "Manage cluster resource pools",
	Args:  cobra.NoArgs,
}

var poolListCmd = &cobra.Command{
	Use:   "list <cluster-id>",
	Short: "List resource pools for a cluster",
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
		pools, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.ResourcePool, bool, error) {
			resp, _, err := c.KubernetesAPI.KubernetesClustersResourcePoolsList(cmd.Context(), id).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing pools: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, pools, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "PACKAGE", "SIZE", "NODES")
			for _, p := range pools {
				output.PrintRow(tw, p.Id, p.Package, p.Size, len(p.Nodes))
			}
			tw.Flush()
		})
	},
}

var (
	poolCreatePkg     string
	poolCreateSize    int32
	poolCreateWait    bool
	poolCreateTimeout time.Duration
)

var poolCreateCmd = &cobra.Command{
	Use:   "create <cluster-id>",
	Short: "Create a resource pool in a cluster",
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
		body := *pidginhost.NewResourcePoolAdd(poolCreatePkg, poolCreateSize)
		resp, _, err := c.KubernetesAPI.KubernetesClustersResourcePoolsCreate(cmd.Context(), id).ResourcePoolAdd(body).Execute()
		if err != nil {
			return fmt.Errorf("creating pool: %w", err)
		}
		cmd.Printf("Resource pool created (ID: %d)\n", resp.Id)

		if poolCreateWait {
			if err := waitForCluster(cmd.Context(), args[0], poolCreateTimeout, cmd); err != nil {
				return err
			}
			cmd.Printf("Cluster %s is active.\n", args[0])
		}
		return nil
	},
}

var poolDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <pool-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a resource pool",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterId, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		if _, err := cmdutil.ParseInt32(args[1]); err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete resource pool %s from cluster %d?", args[1], clusterId)) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersResourcePoolsDestroy(cmd.Context(), clusterId, args[1]).Execute()
		if err != nil {
			return fmt.Errorf("deleting pool: %w", err)
		}
		cmd.Printf("Resource pool %s deleted.\n", args[1])
		return nil
	},
}

// --- Pool Nodes ---

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage resource pool nodes",
	Args:  cobra.NoArgs,
}

var nodeListCmd = &cobra.Command{
	Use:   "list <cluster-id> <pool-id>",
	Short: "List nodes in a resource pool",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterId, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		poolId, err := cmdutil.ParseInt32(args[1])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		nodes, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.ResourcePoolNode, bool, error) {
			resp, _, err := c.KubernetesAPI.KubernetesClustersResourcePoolsNodesList(cmd.Context(), clusterId, poolId).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing nodes: %w", err)
		}
		format := cmdutil.OutputFormat(cmd)
		return output.Print(cmd.OutOrStdout(), format, nodes, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "IP")
			for _, n := range nodes {
				output.PrintRow(tw, n.Id, n.Name, n.Ip)
			}
			tw.Flush()
		})
	},
}

var nodeDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <pool-id> <node-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a node from a resource pool",
	Args:    cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterId, err := cmdutil.ParseInt32(args[0])
		if err != nil {
			return err
		}
		poolId, err := cmdutil.ParseInt32(args[1])
		if err != nil {
			return err
		}
		if _, err := cmdutil.ParseInt32(args[2]); err != nil {
			return err
		}
		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Delete node %s from pool %d?", args[2], poolId)) {
			return nil
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersResourcePoolsNodesDestroy(cmd.Context(), clusterId, args[2], poolId).Execute()
		if err != nil {
			return fmt.Errorf("deleting node: %w", err)
		}
		cmd.Printf("Node %s deleted.\n", args[2])
		return nil
	},
}

func init() {
	clusterCreateCmd.Flags().StringVar(&clusterCreateName, "name", "", "Cluster name")
	clusterCreateCmd.Flags().StringVar(&clusterCreateType, "type", "", "Cluster type (required)")
	clusterCreateCmd.Flags().StringVar(&clusterCreatePkg, "package", "", "Resource pool package (required)")
	clusterCreateCmd.Flags().Int32Var(&clusterCreateSize, "pool-size", 0, "Resource pool size")
	clusterCreateCmd.Flags().StringVar(&clusterCreateVersion, "kube-version", "", "Kubernetes version")
	clusterCreateCmd.Flags().BoolVar(&clusterCreateWait, "wait", false, "Wait for the cluster to become active")
	clusterCreateCmd.Flags().DurationVar(&clusterCreateTimeout, "wait-timeout", defaultWaitTimeout, "Timeout for --wait")
	clusterCreateCmd.MarkFlagRequired("type")
	clusterCreateCmd.MarkFlagRequired("package")

	clusterKubeconfigCmd.Flags().BoolVar(&kubeconfigMerge, "merge", false, "Merge into existing kubeconfig (~/.kube/config or $KUBECONFIG)")

	clusterUpgradeKubeCmd.Flags().BoolVar(&upgradeKubeWait, "wait", false, "Wait for the upgrade to complete")
	clusterUpgradeKubeCmd.Flags().DurationVar(&upgradeKubeTimeout, "wait-timeout", defaultWaitTimeout, "Timeout for --wait")

	clusterUpgradeTalosCmd.Flags().BoolVar(&upgradeTalosWait, "wait", false, "Wait for the upgrade to complete")
	clusterUpgradeTalosCmd.Flags().DurationVar(&upgradeTalosTimeout, "wait-timeout", defaultWaitTimeout, "Timeout for --wait")

	clusterConnectVMCmd.Flags().Int32Var(&connectVMServer, "server", 0, "Server ID to connect (required)")
	clusterConnectVMCmd.MarkFlagRequired("server")

	clusterDisconnectVMCmd.Flags().Int32Var(&disconnectVMServer, "server", 0, "Server ID to disconnect (required)")
	clusterDisconnectVMCmd.MarkFlagRequired("server")

	poolCreateCmd.Flags().StringVar(&poolCreatePkg, "package", "", "Package slug (required)")
	poolCreateCmd.Flags().Int32Var(&poolCreateSize, "size", 0, "Pool size (required)")
	poolCreateCmd.Flags().BoolVar(&poolCreateWait, "wait", false, "Wait for the cluster to become active after pool creation")
	poolCreateCmd.Flags().DurationVar(&poolCreateTimeout, "wait-timeout", defaultWaitTimeout, "Timeout for --wait")
	poolCreateCmd.MarkFlagRequired("package")
	poolCreateCmd.MarkFlagRequired("size")

	nodeCmd.AddCommand(nodeListCmd)
	nodeCmd.AddCommand(nodeDeleteCmd)

	poolCmd.AddCommand(poolListCmd)
	poolCmd.AddCommand(poolCreateCmd)
	poolCmd.AddCommand(poolDeleteCmd)

	clusterCmd.AddCommand(clusterListCmd)
	clusterCmd.AddCommand(clusterGetCmd)
	clusterCmd.AddCommand(clusterCreateCmd)
	clusterCmd.AddCommand(clusterDeleteCmd)
	clusterCmd.AddCommand(clusterKubeconfigCmd)
	clusterCmd.AddCommand(clusterUpgradeKubeCmd)
	clusterCmd.AddCommand(clusterUpgradeTalosCmd)
	clusterCmd.AddCommand(clusterConnectVMCmd)
	clusterCmd.AddCommand(clusterDisconnectVMCmd)
	clusterCmd.AddCommand(clusterConnectedVMsCmd)

	Cmd.AddCommand(clusterCmd)
	Cmd.AddCommand(clusterTypesCmd)
	Cmd.AddCommand(poolCmd)
	Cmd.AddCommand(nodeCmd)
}
