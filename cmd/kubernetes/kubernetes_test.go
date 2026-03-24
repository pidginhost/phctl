package kubernetes

import (
	"testing"
)

func TestKubernetesCommandStructure(t *testing.T) {
	if Cmd.Use != "kubernetes" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "kubernetes")
	}

	aliases := Cmd.Aliases
	found := false
	for _, a := range aliases {
		if a == "k8s" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("kubernetes Aliases = %v, want to contain 'k8s'", aliases)
	}
}

func TestKubernetesSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"cluster", "types", "pool", "node"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestClusterSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range clusterCmd.Commands() {
		names[c.Name()] = true
	}

	expected := []string{"list", "get", "create", "delete", "kubeconfig", "upgrade-kube", "upgrade-talos", "connect-vm", "disconnect-vm", "connected-vms"}
	for _, want := range expected {
		if !names[want] {
			t.Errorf("cluster missing subcommand %q", want)
		}
	}
}

func TestClusterDeleteAliases(t *testing.T) {
	aliases := clusterDeleteCmd.Aliases
	rmFound, destroyFound := false, false
	for _, a := range aliases {
		if a == "rm" {
			rmFound = true
		}
		if a == "destroy" {
			destroyFound = true
		}
	}
	if !rmFound {
		t.Errorf("cluster delete Aliases = %v, want to contain 'rm'", aliases)
	}
	if !destroyFound {
		t.Errorf("cluster delete Aliases = %v, want to contain 'destroy'", aliases)
	}
}

func TestClusterCreateFlags(t *testing.T) {
	for _, name := range []string{"name", "type", "package", "pool-size", "kube-version", "wait", "wait-timeout"} {
		if clusterCreateCmd.Flags().Lookup(name) == nil {
			t.Errorf("cluster create missing flag --%s", name)
		}
	}
}

func TestClusterKubeconfigFlags(t *testing.T) {
	if clusterKubeconfigCmd.Flags().Lookup("merge") == nil {
		t.Error("cluster kubeconfig missing --merge flag")
	}
}

func TestClusterUpgradeKubeFlags(t *testing.T) {
	for _, name := range []string{"wait", "wait-timeout"} {
		if clusterUpgradeKubeCmd.Flags().Lookup(name) == nil {
			t.Errorf("upgrade-kube missing flag --%s", name)
		}
	}
}

func TestClusterUpgradeTalosFlags(t *testing.T) {
	for _, name := range []string{"wait", "wait-timeout"} {
		if clusterUpgradeTalosCmd.Flags().Lookup(name) == nil {
			t.Errorf("upgrade-talos missing flag --%s", name)
		}
	}
}

func TestPoolSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range poolCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "create", "delete"} {
		if !names[want] {
			t.Errorf("pool missing subcommand %q", want)
		}
	}
}

func TestPoolCreateFlags(t *testing.T) {
	for _, name := range []string{"package", "size", "wait", "wait-timeout"} {
		if poolCreateCmd.Flags().Lookup(name) == nil {
			t.Errorf("pool create missing flag --%s", name)
		}
	}
}

func TestNodeSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range nodeCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "delete"} {
		if !names[want] {
			t.Errorf("node missing subcommand %q", want)
		}
	}
}

func TestConnectVMFlags(t *testing.T) {
	if clusterConnectVMCmd.Flags().Lookup("server") == nil {
		t.Error("connect-vm missing --server flag")
	}
}

func TestDisconnectVMFlags(t *testing.T) {
	if clusterDisconnectVMCmd.Flags().Lookup("server") == nil {
		t.Error("disconnect-vm missing --server flag")
	}
}

func TestRouteSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"http-route", "tcp-route", "udp-route"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestHTTPRouteSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range httpRouteCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "create", "delete"} {
		if !names[want] {
			t.Errorf("http-route missing subcommand %q", want)
		}
	}
}

func TestHTTPRouteCreateFlags(t *testing.T) {
	for _, name := range []string{"name", "hostname", "backend", "port", "namespace", "path-prefix", "tls"} {
		if httpRouteCreateCmd.Flags().Lookup(name) == nil {
			t.Errorf("http-route create missing flag --%s", name)
		}
	}
}

func TestTCPRouteSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range tcpRouteCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "create", "delete"} {
		if !names[want] {
			t.Errorf("tcp-route missing subcommand %q", want)
		}
	}
}

func TestTCPRouteCreateFlags(t *testing.T) {
	for _, name := range []string{"name", "port", "backend", "backend-port", "namespace"} {
		if tcpRouteCreateCmd.Flags().Lookup(name) == nil {
			t.Errorf("tcp-route create missing flag --%s", name)
		}
	}
}

func TestUDPRouteSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range udpRouteCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"list", "create", "delete"} {
		if !names[want] {
			t.Errorf("udp-route missing subcommand %q", want)
		}
	}
}

func TestUDPRouteCreateFlags(t *testing.T) {
	for _, name := range []string{"name", "port", "backend", "backend-port", "namespace"} {
		if udpRouteCreateCmd.Flags().Lookup(name) == nil {
			t.Errorf("udp-route create missing flag --%s", name)
		}
	}
}
