package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/cmd/account"
	"github.com/pidginhost/phctl/cmd/auth"
	"github.com/pidginhost/phctl/cmd/billing"
	"github.com/pidginhost/phctl/cmd/compute"
	"github.com/pidginhost/phctl/cmd/dedicated"
	"github.com/pidginhost/phctl/cmd/domain"
	"github.com/pidginhost/phctl/cmd/freedns"
	"github.com/pidginhost/phctl/cmd/hosting"
	"github.com/pidginhost/phctl/cmd/kubernetes"
	"github.com/pidginhost/phctl/cmd/support"
	"github.com/pidginhost/phctl/cmd/update"
	"github.com/pidginhost/phctl/internal/config"
	iupdate "github.com/pidginhost/phctl/internal/update"
)

var rootCmd = &cobra.Command{
	Use:           "phctl",
	Short:         "PidginHost command-line interface",
	Long:          "phctl is a CLI for managing PidginHost cloud resources.",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if cfg, err := config.Load(); err == nil && !cfg.UpdateCheckEnabled() {
			return
		}
		if notice := iupdate.CheckNotice(cmd.Root().Version); notice != "" {
			fmt.Fprint(os.Stderr, notice)
		}
	},
}

func SetVersion(v string) {
	rootCmd.Version = v
	update.SetVersion(v)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		rootCmd.PrintErrln("Error:", err)
		os.Exit(1)
	}
}

func init() {
	outputDefault := "table"
	if cfg, err := config.Load(); err == nil && cfg.Output != "" {
		outputDefault = cfg.Output
	}

	rootCmd.PersistentFlags().StringP("output", "o", outputDefault, "Output format: table, json, yaml")
	rootCmd.PersistentFlags().BoolP("force", "f", false, "Skip confirmation prompts")

	rootCmd.AddCommand(auth.Cmd)
	rootCmd.AddCommand(compute.Cmd)
	rootCmd.AddCommand(account.Cmd)
	rootCmd.AddCommand(domain.Cmd)
	rootCmd.AddCommand(kubernetes.Cmd)
	rootCmd.AddCommand(billing.Cmd)
	rootCmd.AddCommand(dedicated.Cmd)
	rootCmd.AddCommand(freedns.Cmd)
	rootCmd.AddCommand(hosting.Cmd)
	rootCmd.AddCommand(support.Cmd)
	rootCmd.AddCommand(update.Cmd)
}
