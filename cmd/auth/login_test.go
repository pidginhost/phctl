package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestBrowserLoginReturnsPollStatusErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/cli-session/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(cliSessionCreateResponse{
				SessionID:       "session-123",
				VerificationURL: "https://example.test/verify",
			})
		case "/api/auth/cli-session/session-123/":
			http.Error(w, "upstream failed", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldClient := newLoginClient
	oldOpen := openBrowserFunc
	oldInterval := loginPollInterval
	oldTimeout := loginWaitTimeout
	t.Cleanup(func() {
		newLoginClient = oldClient
		openBrowserFunc = oldOpen
		loginPollInterval = oldInterval
		loginWaitTimeout = oldTimeout
	})

	newLoginClient = func() *http.Client { return server.Client() }
	openBrowserFunc = func(string) error { return nil }
	loginPollInterval = 0
	loginWaitTimeout = time.Second

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PIDGINHOST_API_TOKEN", "")
	t.Setenv("PIDGINHOST_API_URL", server.URL)

	cmd := &cobra.Command{Use: "login"}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := browserLogin(cmd)
	if err == nil {
		t.Fatal("browserLogin() error = nil, want poll status failure")
	}
	if !strings.Contains(err.Error(), "unexpected status 502") {
		t.Fatalf("browserLogin() error = %q, want 502 context", err)
	}
	if !strings.Contains(err.Error(), "upstream failed") {
		t.Fatalf("browserLogin() error = %q, want response body", err)
	}
}
