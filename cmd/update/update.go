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
	iupdate.SetVersion(v)
}

// latestRelease is a package-level seam so tests can stub the GitHub call.
var latestRelease = iupdate.LatestRelease

var Cmd = &cobra.Command{
	Use:   "update",
	Short: "Update phctl to the latest version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := iupdate.EnsureSelfUpdateSupported(); err != nil {
			return err
		}

		cmd.Println("Checking for updates...")
		rel, err := latestRelease(iupdate.UpdateTimeout)
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}
		_ = iupdate.RecordCheck()

		if iupdate.IsDevBuild(version) {
			cmd.Printf("Running a development build (version=%q); latest release is %s.\n", version, rel.TagName)
			cmd.Println("Refusing to overwrite a non-release binary. Install a tagged release if you want auto-updates.")
			return nil
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
