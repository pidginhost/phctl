package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/config"
)

var loginToken string

const loginRequestTimeout = 10 * time.Second

var (
	loginPollInterval = 5 * time.Second
	loginWaitTimeout  = 10 * time.Minute
	newLoginClient    = func() *http.Client {
		return &http.Client{Timeout: loginRequestTimeout}
	}
	openBrowserFunc = openBrowser
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with PidginHost",
	Long: `Authenticate with PidginHost using one of these methods:

  Interactive (browser):  phctl auth login
  Direct token:           phctl auth login --token <token>
  Environment variable:   PIDGINHOST_API_TOKEN=<token> phctl ...`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if loginToken != "" {
			return saveToken(cmd, loginToken)
		}
		return browserLogin(cmd)
	},
}

func saveToken(cmd *cobra.Command, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}
	if err := config.Save(&config.Config{AuthToken: token}); err != nil {
		return err
	}
	cmd.Println("Authentication configured successfully.")
	return nil
}

type cliSessionCreateResponse struct {
	SessionID       string `json:"session_id"`
	VerificationURL string `json:"verification_url"`
}

type cliSessionPollResponse struct {
	Status   string `json:"status"`
	TokenKey string `json:"token_key,omitempty"`
}

func browserLogin(cmd *cobra.Command) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	apiURL := strings.TrimRight(cfg.APIURL, "/")
	client := newLoginClient()

	// Create CLI session
	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/api/auth/cli-session/", nil)
	if err != nil {
		return fmt.Errorf("creating CLI session request: %w", err)
	}
	resp, err := client.Do(createReq)
	if err != nil {
		return fmt.Errorf("creating CLI session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d creating CLI session", resp.StatusCode)
	}

	var session cliSessionCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return fmt.Errorf("decoding session response: %w", err)
	}

	verificationURL := session.VerificationURL
	if verificationURL == "" {
		verificationURL = fmt.Sprintf("%s/panel/account/cli-auth/%s/", apiURL, session.SessionID)
	}

	cmd.Printf("Opening browser to: %s\n", verificationURL)
	cmd.Println("If the browser doesn't open, please visit the URL manually.")
	cmd.Println()
	cmd.Println("Waiting for approval...")

	_ = openBrowserFunc(verificationURL)

	// Poll for approval
	pollURL := fmt.Sprintf("%s/api/auth/cli-session/%s/", apiURL, session.SessionID)
	deadline := time.Now().Add(loginWaitTimeout)

	const maxPollErrors = 3
	var consecutiveErrors int
	wait := loginPollInterval
	if wait <= 0 {
		wait = time.Millisecond
	}

	for time.Now().Before(deadline) {
		if err := waitForLoginPoll(ctx, wait, deadline); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				break
			}
			return err
		}

		pollReq, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
		if err != nil {
			return fmt.Errorf("creating login poll request: %w", err)
		}
		pollResp, err := client.Do(pollReq)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return ctx.Err()
			}
			consecutiveErrors++
			if consecutiveErrors >= maxPollErrors {
				return fmt.Errorf("polling login status after %d retries: %w", consecutiveErrors, err)
			}
			continue
		}
		if pollResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(pollResp.Body, 1024))
			pollResp.Body.Close()
			msg := strings.TrimSpace(string(body))
			if msg == "" {
				return fmt.Errorf("polling login status: unexpected status %d", pollResp.StatusCode)
			}
			return fmt.Errorf("polling login status: unexpected status %d: %s", pollResp.StatusCode, msg)
		}

		var poll cliSessionPollResponse
		if err := json.NewDecoder(pollResp.Body).Decode(&poll); err != nil {
			pollResp.Body.Close()
			consecutiveErrors++
			if consecutiveErrors >= maxPollErrors {
				return fmt.Errorf("decoding login response after %d retries: %w", consecutiveErrors, err)
			}
			continue
		}
		pollResp.Body.Close()
		consecutiveErrors = 0 // reset on success

		switch poll.Status {
		case "approved":
			if poll.TokenKey == "" {
				return fmt.Errorf("session approved but no token received")
			}
			if err := saveToken(cmd, poll.TokenKey); err != nil {
				return err
			}
			return nil
		case "denied":
			return fmt.Errorf("login request was denied")
		case "expired":
			return fmt.Errorf("login session expired")
		}
		// "pending" — keep polling
	}

	return fmt.Errorf("login timed out after 10 minutes")
}

func waitForLoginPoll(ctx context.Context, interval time.Duration, deadline time.Time) error {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return context.DeadlineExceeded
	}
	if interval > remaining {
		interval = remaining
	}

	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

func init() {
	loginCmd.Flags().StringVar(&loginToken, "token", "", "API token (for CI/CD and non-interactive use)")
}
