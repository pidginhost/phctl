package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/config"
)

var loginToken string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with PidginHost",
	Long: `Authenticate with PidginHost using one of these methods:

  Interactive (browser):  phctl auth login
  Direct token:           phctl auth login --token <token>
  Environment variable:   PIDGINHOST_API_TOKEN=<token> phctl ...`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if loginToken != "" {
			return saveToken(loginToken)
		}
		return browserLogin()
	},
}

func saveToken(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}
	if err := config.Save(&config.Config{AuthToken: token}); err != nil {
		return err
	}
	fmt.Println("Authentication configured successfully.")
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

func browserLogin() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	apiURL := strings.TrimRight(cfg.APIURL, "/")

	// Create CLI session
	resp, err := http.Post(apiURL+"/api/auth/cli-session/", "application/json", nil)
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

	fmt.Printf("Opening browser to: %s\n", verificationURL)
	fmt.Println("If the browser doesn't open, please visit the URL manually.")
	fmt.Println()
	fmt.Println("Waiting for approval...")

	_ = openBrowser(verificationURL)

	// Poll for approval
	pollURL := fmt.Sprintf("%s/api/auth/cli-session/%s/", apiURL, session.SessionID)
	client := &http.Client{Timeout: 10 * time.Second}
	deadline := time.Now().Add(10 * time.Minute)

	for time.Now().Before(deadline) {
		time.Sleep(5 * time.Second)

		pollResp, err := client.Get(pollURL)
		if err != nil {
			continue
		}

		var poll cliSessionPollResponse
		if err := json.NewDecoder(pollResp.Body).Decode(&poll); err != nil {
			pollResp.Body.Close()
			continue
		}
		pollResp.Body.Close()

		switch poll.Status {
		case "approved":
			if poll.TokenKey == "" {
				return fmt.Errorf("session approved but no token received")
			}
			if err := saveToken(poll.TokenKey); err != nil {
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
