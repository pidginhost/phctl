package kubernetes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"
)

func TestClusterKubeconfigHonorsJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/kubernetes/clusters/123/kubeconfig/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("apiVersion: v1\nclusters: []\n"))
	}))
	defer server.Close()

	oldNewClient := newClient
	t.Cleanup(func() {
		newClient = oldNewClient
	})

	newClient = func() (*pidginhost.APIClient, error) {
		return pidginhost.New("test-token", server.URL), nil
	}

	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().String("output", "table", "")
	root.AddCommand(Cmd)
	if err := root.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatalf("setting output flag: %v", err)
	}

	var out bytes.Buffer
	clusterKubeconfigCmd.SetOut(&out)

	if err := clusterKubeconfigCmd.RunE(clusterKubeconfigCmd, []string{"123"}); err != nil {
		t.Fatalf("clusterKubeconfigCmd.RunE() error: %v", err)
	}

	var got string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("expected JSON string output, got %q: %v", out.String(), err)
	}
	if got != "apiVersion: v1\nclusters: []\n" {
		t.Fatalf("kubeconfig output = %q, want raw kubeconfig string", got)
	}
}
