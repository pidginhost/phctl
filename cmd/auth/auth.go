package auth

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pidginhost/phctl/internal/config"
)

var Cmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize authentication with an API token",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print("Enter your PidginHost API token: ")
		tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
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

		fmt.Println("Authentication configured successfully.")
		return nil
	},
}

var setCmd = &cobra.Command{
	Use:   "set <token>",
	Short: "Set API token directly",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Save(&config.Config{AuthToken: args[0]}); err != nil {
			return err
		}
		fmt.Println("Token saved.")
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
			fmt.Println("Not authenticated. Run 'phctl auth init' to configure.")
		} else {
			masked := maskToken(cfg.AuthToken)
			fmt.Printf("Authenticated (token: %s)\n", masked)
			fmt.Printf("API URL: %s\n", cfg.APIURL)
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
