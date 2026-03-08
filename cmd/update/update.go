package update

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	iupdate "github.com/pidginhost/phctl/internal/update"
)

var version string

func SetVersion(v string) {
	version = v
}

var Cmd = &cobra.Command{
	Use:   "update",
	Short: "Update phctl to the latest version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := iupdate.EnsureSelfUpdateSupported(); err != nil {
			return err
		}

		cmd.Println("Checking for updates...")
		rel, err := iupdate.LatestRelease(iupdate.UpdateTimeout)
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}
		if err := iupdate.RecordCheck(); err != nil {
			return err
		}

		if !iupdate.IsNewer(version, rel.TagName) {
			cmd.Printf("Already up to date (%s).\n", version)
			return nil
		}

		if !cmdutil.Force(cmd) && !confirm.Action(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Update phctl from %s to %s?", version, rel.TagName)) {
			cmd.Println("Update cancelled.")
			return nil
		}

		cmd.Printf("Downloading %s...\n", rel.TagName)
		tmpPath, err := iupdate.DownloadAsset(rel)
		if err != nil {
			return fmt.Errorf("downloading update: %w", err)
		}

		if err := iupdate.Apply(tmpPath); err != nil {
			return fmt.Errorf("applying update: %w", err)
		}

		cmd.Printf("Updated to %s.\n", rel.TagName)
		return nil
	},
}
