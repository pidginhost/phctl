package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pidginhost/phctl/internal/client"
)

var newClient = client.New

const (
	defaultWaitTimeout = 10 * time.Minute
	waitPollInterval   = 15 * time.Second
)

// waitForCluster polls a cluster's status until it becomes "active" or a
// terminal error state is reached. It respects context cancellation (Ctrl+C).
func waitForCluster(ctx context.Context, clusterID string, timeout time.Duration, cmd *cobra.Command) error {
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out waiting for cluster %s to become active", clusterID)
		}

		interval := waitPollInterval
		if interval > remaining {
			interval = remaining
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		var cl client.RawCluster
		if err := client.RawGet(ctx, fmt.Sprintf("/api/kubernetes/clusters/%s/", clusterID), &cl); err != nil {
			return fmt.Errorf("polling cluster status: %w", err)
		}

		cmd.PrintErrln("Cluster " + clusterID + ": " + cl.Status)

		switch cl.Status {
		case "active":
			return nil
		case "failed", "error", "cancelled":
			return fmt.Errorf("cluster %s entered %q state", clusterID, cl.Status)
		}
		// provisioning, upgrading, etc. — keep polling
	}
}

// --- Kubeconfig merge ---

func kubeconfigPath() string {
	if p := os.Getenv("KUBECONFIG"); p != "" {
		// Use first path if KUBECONFIG contains multiple entries.
		return strings.SplitN(p, string(os.PathListSeparator), 2)[0]
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube", "config")
}

// mergeKubeconfig parses the incoming kubeconfig YAML, merges it into the
// existing kubeconfig file (default ~/.kube/config), sets the new context
// as current, and writes the result back.
//
// The existing file is unmarshalled as a generic map so that all top-level
// keys (extensions, preferences, etc.) are preserved even if this code does
// not explicitly model them.
func mergeKubeconfig(rawYAML string) (string, error) {
	var incoming map[string]interface{}
	if err := yaml.Unmarshal([]byte(rawYAML), &incoming); err != nil {
		return "", fmt.Errorf("parsing incoming kubeconfig: %w", err)
	}

	kubePath := kubeconfigPath()

	existing := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Config",
	}
	if data, err := os.ReadFile(kubePath); err == nil {
		if err := yaml.Unmarshal(data, &existing); err != nil {
			return "", fmt.Errorf("parsing existing kubeconfig %s: %w", kubePath, err)
		}
	}

	// Merge only the standard named arrays; leave all other keys untouched.
	for _, key := range []string{"clusters", "contexts", "users"} {
		existing[key] = mergeNamedEntries(toMapSlice(existing[key]), toMapSlice(incoming[key]))
	}
	if cc, ok := incoming["current-context"].(string); ok && cc != "" {
		existing["current-context"] = cc
	}

	if err := os.MkdirAll(filepath.Dir(kubePath), 0700); err != nil {
		return "", err
	}
	data, err := yaml.Marshal(existing)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(kubePath, data, 0600); err != nil {
		return "", err
	}
	return kubePath, nil
}

// toMapSlice converts the []interface{} that yaml.Unmarshal produces for
// arrays inside map[string]interface{} into []map[string]interface{}.
func toMapSlice(v interface{}) []map[string]interface{} {
	if v == nil {
		return nil
	}
	slice, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(slice))
	for _, item := range slice {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

// mergeNamedEntries upserts incoming entries into existing by the "name" key.
func mergeNamedEntries(existing, incoming []map[string]interface{}) []map[string]interface{} {
	byName := make(map[string]int)
	for i, e := range existing {
		if name, ok := e["name"].(string); ok {
			byName[name] = i
		}
	}
	for _, e := range incoming {
		name, ok := e["name"].(string)
		if !ok {
			existing = append(existing, e)
			continue
		}
		if idx, found := byName[name]; found {
			existing[idx] = e
		} else {
			existing = append(existing, e)
			byName[name] = len(existing) - 1
		}
	}
	return existing
}
