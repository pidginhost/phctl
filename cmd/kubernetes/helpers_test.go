package kubernetes

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMergeNamedEntries(t *testing.T) {
	existing := []map[string]interface{}{
		{"name": "cluster-a", "server": "https://a.example.com"},
		{"name": "cluster-b", "server": "https://b.example.com"},
	}
	incoming := []map[string]interface{}{
		{"name": "cluster-b", "server": "https://b-new.example.com"},
		{"name": "cluster-c", "server": "https://c.example.com"},
	}

	result := mergeNamedEntries(existing, incoming)

	if len(result) != 3 {
		t.Fatalf("got %d entries, want 3", len(result))
	}

	// cluster-a unchanged
	if result[0]["server"] != "https://a.example.com" {
		t.Errorf("cluster-a server = %v, want https://a.example.com", result[0]["server"])
	}
	// cluster-b replaced
	if result[1]["server"] != "https://b-new.example.com" {
		t.Errorf("cluster-b server = %v, want https://b-new.example.com", result[1]["server"])
	}
	// cluster-c appended
	if result[2]["name"] != "cluster-c" {
		t.Errorf("result[2] name = %v, want cluster-c", result[2]["name"])
	}
}

func TestMergeNamedEntriesEmpty(t *testing.T) {
	result := mergeNamedEntries(nil, []map[string]interface{}{
		{"name": "new", "data": "value"},
	})
	if len(result) != 1 {
		t.Fatalf("got %d entries, want 1", len(result))
	}
}

func TestToMapSlice(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"name": "a"},
		map[string]interface{}{"name": "b"},
	}
	result := toMapSlice(input)
	if len(result) != 2 {
		t.Fatalf("got %d, want 2", len(result))
	}
}

func TestToMapSliceNil(t *testing.T) {
	result := toMapSlice(nil)
	if result != nil {
		t.Fatalf("got %v, want nil", result)
	}
}

func TestMergeKubeconfig(t *testing.T) {
	// Set up temp dir as KUBECONFIG target
	tmp := t.TempDir()
	kubePath := filepath.Join(tmp, "config")
	t.Setenv("KUBECONFIG", kubePath)

	// Write an existing kubeconfig with an extra top-level key
	existing := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://old.example.com
  name: old-cluster
contexts:
- context:
    cluster: old-cluster
    user: old-user
  name: old-context
users:
- name: old-user
  user:
    token: old-token
current-context: old-context
my-custom-extension: preserved-value
`
	if err := os.WriteFile(kubePath, []byte(existing), 0600); err != nil {
		t.Fatalf("writing existing kubeconfig: %v", err)
	}

	// Merge new cluster
	incoming := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://new.example.com
    certificate-authority-data: AAAA
  name: new-cluster
contexts:
- context:
    cluster: new-cluster
    user: new-user
  name: new-context
users:
- name: new-user
  user:
    token: new-token
current-context: new-context
`
	path, err := mergeKubeconfig(incoming)
	if err != nil {
		t.Fatalf("mergeKubeconfig: %v", err)
	}
	if path != kubePath {
		t.Errorf("path = %q, want %q", path, kubePath)
	}

	// Read back and verify
	data, err := os.ReadFile(kubePath)
	if err != nil {
		t.Fatalf("reading merged kubeconfig: %v", err)
	}

	var merged map[string]interface{}
	if err := yaml.Unmarshal(data, &merged); err != nil {
		t.Fatalf("parsing merged kubeconfig: %v", err)
	}

	clusters := toMapSlice(merged["clusters"])
	contexts := toMapSlice(merged["contexts"])
	users := toMapSlice(merged["users"])

	if len(clusters) != 2 {
		t.Errorf("got %d clusters, want 2", len(clusters))
	}
	if len(contexts) != 2 {
		t.Errorf("got %d contexts, want 2", len(contexts))
	}
	if len(users) != 2 {
		t.Errorf("got %d users, want 2", len(users))
	}
	if merged["current-context"] != "new-context" {
		t.Errorf("current-context = %v, want %q", merged["current-context"], "new-context")
	}
	// Verify unknown top-level key was preserved
	if merged["my-custom-extension"] != "preserved-value" {
		t.Errorf("my-custom-extension = %v, want %q", merged["my-custom-extension"], "preserved-value")
	}
}

func TestMergeKubeconfigCreatesFile(t *testing.T) {
	tmp := t.TempDir()
	kubePath := filepath.Join(tmp, "subdir", "config")
	t.Setenv("KUBECONFIG", kubePath)

	incoming := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://new.example.com
  name: new-cluster
contexts:
- context:
    cluster: new-cluster
    user: new-user
  name: new-context
users:
- name: new-user
  user:
    token: new-token
current-context: new-context
`
	path, err := mergeKubeconfig(incoming)
	if err != nil {
		t.Fatalf("mergeKubeconfig: %v", err)
	}
	if path != kubePath {
		t.Errorf("path = %q, want %q", path, kubePath)
	}

	data, err := os.ReadFile(kubePath)
	if err != nil {
		t.Fatalf("reading kubeconfig: %v", err)
	}
	var merged map[string]interface{}
	if err := yaml.Unmarshal(data, &merged); err != nil {
		t.Fatalf("parsing kubeconfig: %v", err)
	}
	clusters := toMapSlice(merged["clusters"])
	if len(clusters) != 1 {
		t.Errorf("got %d clusters, want 1", len(clusters))
	}
}

func TestMergeKubeconfigReplaceExisting(t *testing.T) {
	tmp := t.TempDir()
	kubePath := filepath.Join(tmp, "config")
	t.Setenv("KUBECONFIG", kubePath)

	// Write existing with a cluster named "shared"
	existing := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://old.example.com
  name: shared
contexts: []
users: []
current-context: ""
`
	if err := os.WriteFile(kubePath, []byte(existing), 0600); err != nil {
		t.Fatalf("writing existing: %v", err)
	}

	// Merge with updated "shared" cluster
	incoming := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://updated.example.com
  name: shared
contexts: []
users: []
current-context: ""
`
	if _, err := mergeKubeconfig(incoming); err != nil {
		t.Fatalf("mergeKubeconfig: %v", err)
	}

	data, err := os.ReadFile(kubePath)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	var merged map[string]interface{}
	if err := yaml.Unmarshal(data, &merged); err != nil {
		t.Fatalf("parsing: %v", err)
	}
	clusters := toMapSlice(merged["clusters"])
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1 (should replace, not append)", len(clusters))
	}
	cluster := clusters[0]["cluster"].(map[string]interface{})
	if cluster["server"] != "https://updated.example.com" {
		t.Errorf("server = %v, want https://updated.example.com", cluster["server"])
	}
}

func TestMergeKubeconfigPreservesExistingOnWriteFailure(t *testing.T) {
	tmp := t.TempDir()
	kubePath := filepath.Join(tmp, "config")
	t.Setenv("KUBECONFIG", kubePath)

	existing := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://old.example.com
  name: old-cluster
contexts: []
users: []
current-context: ""
`
	if err := os.WriteFile(kubePath, []byte(existing), 0600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	before, err := os.ReadFile(kubePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	old := writeAtomic
	writeAtomic = func(string, []byte, os.FileMode) error {
		return errors.New("simulated atomic write failure")
	}
	t.Cleanup(func() { writeAtomic = old })

	incoming := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://new.example.com
  name: new-cluster
contexts: []
users: []
current-context: ""
`
	if _, err := mergeKubeconfig(incoming); err == nil {
		t.Fatal("expected error when atomic write fails")
	}

	after, err := os.ReadFile(kubePath)
	if err != nil {
		t.Fatalf("ReadFile after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("kubeconfig was modified despite write failure\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestKubeconfigPathDefault(t *testing.T) {
	t.Setenv("KUBECONFIG", "")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".kube", "config")
	got := kubeconfigPath()
	if got != want {
		t.Errorf("kubeconfigPath() = %q, want %q", got, want)
	}
}

func TestKubeconfigPathFromEnv(t *testing.T) {
	t.Setenv("KUBECONFIG", "/custom/path/config:/other/path")
	got := kubeconfigPath()
	if got != "/custom/path/config" {
		t.Errorf("kubeconfigPath() = %q, want %q", got, "/custom/path/config")
	}
}
