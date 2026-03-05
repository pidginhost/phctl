package kubernetes

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

var Cmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"k8s"},
	Short:   "Manage Kubernetes clusters",
}

// --- Clusters ---

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage Kubernetes clusters",
}

var clusterListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all clusters",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing clusters: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, resp.Results, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "STATUS", "TYPE", "KUBE VERSION", "IPV4")
			for _, cl := range resp.Results {
				output.PrintRow(tw, cl.Id, pstr(cl.Name), cl.Status, cl.ClusterType, cl.KubeVersion, cl.Ipv4Address)
			}
			tw.Flush()
		})
		return nil
	},
}

var clusterGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get cluster details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		cl, _, err := c.KubernetesAPI.KubernetesClustersRetrieve(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("getting cluster: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, cl, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", cl.Id)
			output.PrintRow(tw, "Name:", pstr(cl.Name))
			output.PrintRow(tw, "Status:", cl.Status)
			output.PrintRow(tw, "Type:", cl.ClusterType)
			output.PrintRow(tw, "Kube Version:", cl.KubeVersion)
			output.PrintRow(tw, "Talos Version:", cl.TalosVersion)
			output.PrintRow(tw, "IPv4:", cl.Ipv4Address)
			output.PrintRow(tw, "Price/Month:", cl.PricePerMonth)
			output.PrintRow(tw, "Features Ready:", cl.FeaturesReady)
			tw.Flush()
		})
		return nil
	},
}

var (
	clusterCreateName    string
	clusterCreateType    string
	clusterCreatePkg     string
	clusterCreateSize    int32
	clusterCreateVersion string
)

var clusterCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a Kubernetes cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
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

		resp, _, err := c.KubernetesAPI.KubernetesClustersCreate(context.Background()).ClusterAdd(body).Execute()
		if err != nil {
			return fmt.Errorf("creating cluster: %w", err)
		}
		fmt.Printf("Cluster created (ID: %d)\n", resp.Id)
		return nil
	},
}

var clusterDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"destroy", "rm"},
	Short:   "Delete a cluster",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete cluster %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersDestroy(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("deleting cluster: %w", err)
		}
		fmt.Printf("Cluster %s deleted.\n", args[0])
		return nil
	},
}

var clusterKubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig <id>",
	Short: "Get cluster kubeconfig",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersKubeconfigRetrieve(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("getting kubeconfig: %w", err)
		}
		fmt.Println(resp)
		return nil
	},
}

var clusterUpgradeKubeCmd = &cobra.Command{
	Use:   "upgrade-kube <id>",
	Short: "Upgrade Kubernetes to the next available version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Upgrade Kubernetes version for cluster %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersKubeVersionUpgradeCreate(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("upgrading kube version: %w", err)
		}
		fmt.Printf("Kubernetes upgrade initiated: %s\n", resp.Status)
		return nil
	},
}

var clusterUpgradeTalosCmd = &cobra.Command{
	Use:   "upgrade-talos <id>",
	Short: "Upgrade Talos to the next available version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Upgrade Talos version for cluster %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersTalosVersionUpgradeCreate(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("upgrading talos version: %w", err)
		}
		fmt.Printf("Talos upgrade initiated: %s\n", resp.Status)
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
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewConnectVMRequest(connectVMServer)
		resp, _, err := c.KubernetesAPI.KubernetesClustersConnectVmCreate(context.Background(), args[0]).ConnectVMRequest(body).Execute()
		if err != nil {
			return fmt.Errorf("connecting VM: %w", err)
		}
		fmt.Printf("VM connected: %s - %s\n", resp.Status, resp.Message)
		return nil
	},
}

var disconnectVMServer int32

var clusterDisconnectVMCmd = &cobra.Command{
	Use:   "disconnect-vm <cluster-id>",
	Short: "Disconnect a cloud VM from the cluster private network",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewDisconnectVMRequest(disconnectVMServer)
		resp, _, err := c.KubernetesAPI.KubernetesClustersDisconnectVmCreate(context.Background(), args[0]).DisconnectVMRequest(body).Execute()
		if err != nil {
			return fmt.Errorf("disconnecting VM: %w", err)
		}
		fmt.Printf("VM disconnected: %s - %s\n", resp.Status, resp.Message)
		return nil
	},
}

var clusterConnectedVMsCmd = &cobra.Command{
	Use:   "connected-vms <cluster-id>",
	Short: "List VMs connected to the cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersConnectedVmsRetrieve(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("listing connected VMs: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, resp.Vms, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME", "IP")
			for _, vm := range resp.Vms {
				output.PrintRow(tw, vm.Id, vm.Hostname, vm.Ip)
			}
			tw.Flush()
		})
		return nil
	},
}

// --- Cluster Types ---

var clusterTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List available cluster types",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClusterTypesList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing cluster types: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, resp.Results, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "TYPE", "MIN WORKERS", "MAX WORKERS", "PACKAGES")
			for _, t := range resp.Results {
				output.PrintRow(tw, t.Type, pstr(t.WorkerNodesCountMin), pstr(t.WorkerNodesCountMax), len(t.WorkerNodePackages))
			}
			tw.Flush()
		})
		return nil
	},
}

// --- Resource Pools ---

var poolCmd = &cobra.Command{
	Use:   "pool",
	Short: "Manage cluster resource pools",
}

var poolListCmd = &cobra.Command{
	Use:   "list <cluster-id>",
	Short: "List resource pools for a cluster",
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
		resp, _, err := c.KubernetesAPI.KubernetesClustersResourcePoolsList(context.Background(), id).Execute()
		if err != nil {
			return fmt.Errorf("listing pools: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, resp.Results, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "PACKAGE", "SIZE", "NODES")
			for _, p := range resp.Results {
				output.PrintRow(tw, p.Id, p.Package, p.Size, len(p.Nodes))
			}
			tw.Flush()
		})
		return nil
	},
}

var (
	poolCreatePkg  string
	poolCreateSize int32
)

var poolCreateCmd = &cobra.Command{
	Use:   "create <cluster-id>",
	Short: "Create a resource pool in a cluster",
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
		body := *pidginhost.NewResourcePoolAdd(poolCreatePkg, poolCreateSize)
		resp, _, err := c.KubernetesAPI.KubernetesClustersResourcePoolsCreate(context.Background(), id).ResourcePoolAdd(body).Execute()
		if err != nil {
			return fmt.Errorf("creating pool: %w", err)
		}
		fmt.Printf("Resource pool created (ID: %d)\n", resp.Id)
		return nil
	},
}

var poolDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <pool-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a resource pool",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterId, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete resource pool %s from cluster %d?", args[1], clusterId)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersResourcePoolsDestroy(context.Background(), clusterId, args[1]).Execute()
		if err != nil {
			return fmt.Errorf("deleting pool: %w", err)
		}
		fmt.Printf("Resource pool %s deleted.\n", args[1])
		return nil
	},
}

// --- Pool Nodes ---

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage resource pool nodes",
}

var nodeListCmd = &cobra.Command{
	Use:   "list <cluster-id> <pool-id>",
	Short: "List nodes in a resource pool",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterId, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		poolId, err := parseInt32(args[1])
		if err != nil {
			return err
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.KubernetesAPI.KubernetesClustersResourcePoolsNodesList(context.Background(), clusterId, poolId).Execute()
		if err != nil {
			return fmt.Errorf("listing nodes: %w", err)
		}
		format := outputFormat(cmd)
		output.Print(format, resp.Results, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "NAME", "IP")
			for _, n := range resp.Results {
				output.PrintRow(tw, n.Id, n.Name, n.Ip)
			}
			tw.Flush()
		})
		return nil
	},
}

var nodeDeleteCmd = &cobra.Command{
	Use:     "delete <cluster-id> <pool-id> <node-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a node from a resource pool",
	Args:    cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterId, err := parseInt32(args[0])
		if err != nil {
			return err
		}
		poolId, err := parseInt32(args[1])
		if err != nil {
			return err
		}
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete node %s from pool %d?", args[2], poolId)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.KubernetesAPI.KubernetesClustersResourcePoolsNodesDestroy(context.Background(), clusterId, args[2], poolId).Execute()
		if err != nil {
			return fmt.Errorf("deleting node: %w", err)
		}
		fmt.Printf("Node %s deleted.\n", args[2])
		return nil
	},
}

func init() {
	clusterCreateCmd.Flags().StringVar(&clusterCreateName, "name", "", "Cluster name")
	clusterCreateCmd.Flags().StringVar(&clusterCreateType, "type", "", "Cluster type (required)")
	clusterCreateCmd.Flags().StringVar(&clusterCreatePkg, "package", "", "Resource pool package (required)")
	clusterCreateCmd.Flags().Int32Var(&clusterCreateSize, "pool-size", 0, "Resource pool size")
	clusterCreateCmd.Flags().StringVar(&clusterCreateVersion, "kube-version", "", "Kubernetes version")
	clusterCreateCmd.MarkFlagRequired("type")
	clusterCreateCmd.MarkFlagRequired("package")

	clusterConnectVMCmd.Flags().Int32Var(&connectVMServer, "server", 0, "Server ID to connect (required)")
	clusterConnectVMCmd.MarkFlagRequired("server")

	clusterDisconnectVMCmd.Flags().Int32Var(&disconnectVMServer, "server", 0, "Server ID to disconnect (required)")
	clusterDisconnectVMCmd.MarkFlagRequired("server")

	poolCreateCmd.Flags().StringVar(&poolCreatePkg, "package", "", "Package slug (required)")
	poolCreateCmd.Flags().Int32Var(&poolCreateSize, "size", 0, "Pool size (required)")
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
