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
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/config"
	"github.com/pidginhost/phctl/internal/output"
	iupdate "github.com/pidginhost/phctl/internal/update"
)

var rootCmd = &cobra.Command{
	Use:           "phctl",
	Short:         "PidginHost command-line interface",
	Long:          "phctl is a CLI for managing PidginHost cloud resources.",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return validateOutputFlag(cmd)
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if os.Getenv("PHCTL_NO_UPDATE_CHECK") == "1" {
			return
		}
		if cfg, err := config.Load(); err == nil && !cfg.UpdateCheckEnabled() {
			return
		}
		_ = iupdate.StartBackgroundCheck(cmd.Root().Version)
	},
}

func SetVersion(v string) {
	rootCmd.Version = v
	update.SetVersion(v)
}

func Execute() {
	ctx, cancel := cmdutil.SignalContext()
	rootCmd.SetContext(ctx)
	err := rootCmd.Execute()
	cancel()
	if err != nil {
		rootCmd.PrintErrln("Error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("output", "o", defaultOutputFormat(), "Output format: table, json, yaml")
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
	rootCmd.AddCommand(support.TicketCmd)
	rootCmd.AddCommand(update.Cmd)
	rootCmd.AddCommand(update.CheckCmd)

	setDefaultArgs(rootCmd)
}

func defaultOutputFormat() string {
	if cfg, err := config.Load(); err == nil && output.IsValidFormat(cfg.Output) {
		return cfg.Output
	}
	return string(output.FormatTable)
}

func validateOutputFlag(cmd *cobra.Command) error {
	format, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}
	if !output.IsValidFormat(format) {
		return fmt.Errorf("invalid output format %q (expected one of: table, json, yaml)", format)
	}
	return nil
}

func setDefaultArgs(cmd *cobra.Command) {
	if cmd.Args == nil {
		cmd.Args = cobra.NoArgs
	}
	for _, child := range cmd.Commands() {
		setDefaultArgs(child)
	}
}
