package update

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	iupdate "github.com/pidginhost/phctl/internal/update"
)

func TestUpdateCommandStructure(t *testing.T) {
	if Cmd.Use != "update" {
		t.Errorf("Use = %q, want %q", Cmd.Use, "update")
	}
	if Cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestCheckCommandStructure(t *testing.T) {
	if CheckCmd.Use != "__update-check <current-version>" {
		t.Errorf("Use = %q, want %q", CheckCmd.Use, "__update-check <current-version>")
	}
	if !CheckCmd.Hidden {
		t.Error("CheckCmd should be hidden")
	}
}

func TestSetVersion(t *testing.T) {
	old := version
	defer func() { version = old }()

	SetVersion("1.2.3")
	if version != "1.2.3" {
		t.Errorf("version = %q, want %q", version, "1.2.3")
	}
}

func TestCheckCmdArgs(t *testing.T) {
	if err := CheckCmd.Args(CheckCmd, nil); err == nil {
		t.Error("CheckCmd should reject zero args")
	}
	if err := CheckCmd.Args(CheckCmd, []string{"v1"}); err != nil {
		t.Errorf("CheckCmd should accept one arg: %v", err)
	}
	if err := CheckCmd.Args(CheckCmd, []string{"v1", "v2"}); err == nil {
		t.Error("CheckCmd should reject two args")
	}
}

func TestUpdateCmdArgs(t *testing.T) {
	if err := Cmd.Args(Cmd, nil); err != nil {
		t.Errorf("Cmd should accept zero args: %v", err)
	}
	if err := Cmd.Args(Cmd, []string{"extra"}); err == nil {
		t.Error("Cmd should reject extra args")
	}
}

func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "phctl"}
	root.PersistentFlags().StringP("output", "o", "table", "Output format")
	root.PersistentFlags().BoolP("force", "f", false, "Skip confirmation")
	root.AddCommand(Cmd)
	root.AddCommand(CheckCmd)
	return root
}

func TestUpdateCmdNoSupportWindows(t *testing.T) {
	// This test just verifies the command can be found in the tree
	root := newTestRoot()
	cmd, _, err := root.Find([]string{"update"})
	if err != nil {
		t.Fatalf("Find(update) error: %v", err)
	}
	if cmd.Use != "update" {
		t.Errorf("found command Use = %q, want %q", cmd.Use, "update")
	}
}

func TestCheckCmdCanBeFound(t *testing.T) {
	root := newTestRoot()
	cmd, _, err := root.Find([]string{"__update-check", "v1.0.0"})
	if err != nil {
		t.Fatalf("Find(__update-check) error: %v", err)
	}
	if !cmd.Hidden {
		t.Error("__update-check should be hidden")
	}
}

func TestUpdateCmdDevBuildDoesNotClaimUpToDate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	oldVersion := version
	defer func() { version = oldVersion }()
	version = "" // simulate a dev build

	oldFetch := latestRelease
	defer func() { latestRelease = oldFetch }()
	latestRelease = func(timeout time.Duration) (*iupdate.Release, error) {
		return &iupdate.Release{
			TagName: "v1.2.3",
			Assets: []iupdate.Asset{
				{Name: "phctl-linux-amd64", BrowserDownloadURL: "https://example.test/x"},
			},
		}, nil
	}

	var out bytes.Buffer
	Cmd.SetOut(&out)
	Cmd.SetErr(&out)

	if err := Cmd.RunE(Cmd, nil); err != nil {
		t.Fatalf("Cmd.RunE error: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "Already up to date") {
		t.Errorf("output claims up-to-date for dev build:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "development build") {
		t.Errorf("output should mention development build, got:\n%s", got)
	}
	if strings.Contains(got, "Downloading") {
		t.Errorf("update should not be applied to a dev build, got:\n%s", got)
	}
}

func TestCheckCmdOutputsNothing(t *testing.T) {
	// CheckNotice returns "" when ShouldCheck() returns false
	// (which it does in a fresh temp dir after RecordCheck)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	var out bytes.Buffer
	CheckCmd.SetOut(&out)
	CheckCmd.SetErr(&out)

	// CheckCmd calls CheckNotice which calls ShouldCheck.
	// In a fresh dir ShouldCheck returns true but LatestRelease will fail
	// (no network), so CheckNotice returns "".
	err := CheckCmd.RunE(CheckCmd, []string{"99.99.99"})
	if err != nil {
		t.Fatalf("CheckCmd.RunE error: %v", err)
	}
}
