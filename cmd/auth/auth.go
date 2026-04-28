package auth

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pidginhost/phctl/internal/config"
)

var Cmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Args:  cobra.NoArgs,
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize authentication with an API token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := fmt.Fprint(cmd.ErrOrStderr(), "Enter your PidginHost API token: "); err != nil {
			return err
		}
		tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		if _, printErr := io.WriteString(cmd.ErrOrStderr(), "\n"); printErr != nil {
			return printErr
		}
		if err != nil {
			return fmt.Errorf("reading token: %w", err)
		}
		token := strings.TrimSpace(string(tokenBytes))
		if token == "" {
			return fmt.Errorf("token cannot be empty")
		}

		if err := config.Save(&config.Config{AuthToken: token}); err != nil {
			return err
		}

		cmd.Println("Authentication configured successfully.")
		return nil
	},
}

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set API token from stdin",
	Long: `Set the API token by piping it on stdin.

Token is read from stdin to avoid leaking it via the process argument list
(visible in 'ps' output and /proc/<pid>/cmdline). Examples:

  echo "$MY_TOKEN" | phctl auth set
  phctl auth set < token.txt`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(cmd.InOrStdin())
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("reading token from stdin: %w", err)
		}
		token := strings.TrimSpace(line)
		if token == "" {
			return fmt.Errorf("token cannot be empty (pipe it on stdin)")
		}
		if err := config.Save(&config.Config{AuthToken: token}); err != nil {
			return err
		}
		cmd.Println("Token saved.")
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.AuthToken == "" {
			cmd.Println("Not authenticated. Run 'phctl auth init' to configure.")
		} else {
			masked := maskToken(cfg.AuthToken)
			cmd.Printf("Authenticated (token: %s)\n", masked)
			cmd.Printf("API URL: %s\n", cfg.APIURL)
		}
		return nil
	},
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func init() {
	Cmd.AddCommand(initCmd)
	Cmd.AddCommand(setCmd)
	Cmd.AddCommand(statusCmd)
	Cmd.AddCommand(loginCmd)
}
