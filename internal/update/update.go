package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pidginhost/phctl/internal/config"
)

const checksumsAssetName = "checksums.txt"

const (
	repoAPIURL    = "https://api.github.com/repos/pidginhost/phctl"
	backgroundCmd = "__update-check"
	checkInterval = 24 * time.Hour
	CheckTimeout  = 2 * time.Second
	UpdateTimeout = 60 * time.Second
)

var (
	latestReleaseURL = repoAPIURL + "/releases/latest"
	newHTTPClient    = func(timeout time.Duration) *http.Client {
		return &http.Client{Timeout: timeout}
	}
	execPathFunc  = execPath
	execCommand   = exec.Command
	clientVersion = "dev"
)

// SetVersion records the running phctl version so update HTTP requests can
// include a meaningful User-Agent. Safe to call once during initialization.
func SetVersion(v string) {
	if v == "" {
		v = "dev"
	}
	clientVersion = v
}

func userAgent() string {
	return "phctl/" + clientVersion
}

var ErrSelfUpdateUnsupported = errors.New("self-update is not supported on Windows; download the latest phctl release manually")

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func assetName() string {
	name := fmt.Sprintf("phctl-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func supportsSelfUpdate(goos string) bool {
	return goos != "windows"
}

func EnsureSelfUpdateSupported() error {
	if !supportsSelfUpdate(runtime.GOOS) {
		return ErrSelfUpdateUnsupported
	}
	return nil
}

func lastCheckPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "last-update-check"), nil
}

// ShouldCheck reports whether enough time has passed since the last update check.
func ShouldCheck() bool {
	path, err := lastCheckPath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return true // first run or missing file
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return true
	}
	return time.Since(time.Unix(ts, 0)) >= checkInterval
}

// RecordCheck saves the current time so the next check is throttled.
func RecordCheck() error {
	path, err := lastCheckPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating update check directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0600); err != nil {
		return fmt.Errorf("recording update check: %w", err)
	}
	return nil
}

// LatestRelease fetches the most recent release from the GitHub API.
func LatestRelease(timeout time.Duration) (*Release, error) {
	client := newHTTPClient(timeout)
	req, err := http.NewRequest(http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// IsDevBuild reports whether the current version string is unparseable as
// a semver (empty, "dev", a commit SHA, etc.). Such builds did not come
// from a tagged release and the self-update path should not pretend they
// are "up to date" relative to the latest GitHub release.
func IsDevBuild(current string) bool {
	return parseVersion(current) == nil
}

// IsNewer reports whether latest is a newer semver than current.
func IsNewer(current, latest string) bool {
	cur := parseVersion(current)
	lat := parseVersion(latest)
	if cur == nil || lat == nil {
		return false
	}
	if lat[0] != cur[0] {
		return lat[0] > cur[0]
	}
	if lat[1] != cur[1] {
		return lat[1] > cur[1]
	}
	return lat[2] > cur[2]
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	nums := make([]int, 3)
	for i, p := range parts {
		if idx := strings.IndexAny(p, "-+"); idx >= 0 {
			p = p[:idx]
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		nums[i] = n
	}
	return nums
}

// DownloadAsset downloads the release asset matching the current OS/arch,
// verifies its SHA-256 against the release's checksums.txt asset, and returns
// the path to the temporary file. A missing or mismatched checksum aborts the
// update so a tampered or partial download cannot replace the running binary.
func DownloadAsset(rel *Release) (string, error) {
	want := assetName()
	var downloadURL, checksumsURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case want:
			downloadURL = a.BrowserDownloadURL
		case checksumsAssetName:
			checksumsURL = a.BrowserDownloadURL
		}
	}
	if downloadURL == "" {
		return "", fmt.Errorf("no asset found for %s (check release %s has a matching binary)", want, rel.TagName)
	}
	if checksumsURL == "" {
		return "", fmt.Errorf("release %s is missing a %s asset; refusing to install an unverified binary", rel.TagName, checksumsAssetName)
	}

	expectedSum, err := fetchExpectedChecksum(checksumsURL, want)
	if err != nil {
		return "", err
	}

	tmpPath, err := downloadToTempFile(downloadURL)
	if err != nil {
		return "", err
	}

	if err := verifyChecksum(tmpPath, expectedSum); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

func downloadToTempFile(url string) (string, error) {
	client := newHTTPClient(UpdateTimeout)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("preparing download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent())
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	exe, err := execPathFunc()
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(exe), "phctl-update-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("writing update: %w", err)
	}
	if err := tmp.Chmod(0755); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("setting permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("closing temp file: %w", err)
	}
	return tmp.Name(), nil
}

// fetchExpectedChecksum downloads the checksums.txt asset and returns the
// hex-encoded SHA-256 entry for assetName. Lines look like:
//
//	abcd...  phctl-linux-amd64
//
// Comments (#...) and blank lines are skipped. Filenames may have a leading
// '*' (binary mode) which we strip.
func fetchExpectedChecksum(url, asset string) (string, error) {
	client := newHTTPClient(UpdateTimeout)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("preparing checksums request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent())
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading checksums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksums download returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("reading checksums: %w", err)
	}

	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if name == asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum entry for %s in checksums.txt", asset)
}

func verifyChecksum(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening downloaded asset for hashing: %w", err)
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, f)
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("hashing downloaded asset: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing downloaded asset after hashing: %w", closeErr)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, got)
	}
	return nil
}

// Apply replaces the running binary with the file at tmpPath.
func Apply(tmpPath string) error {
	if err := EnsureSelfUpdateSupported(); err != nil {
		return err
	}

	exe, err := execPathFunc()
	if err != nil {
		return err
	}

	backup := exe + ".bak"
	if err := os.Rename(exe, backup); err != nil {
		return fmt.Errorf("backing up current binary: %w", err)
	}

	if err := os.Rename(tmpPath, exe); err != nil {
		_ = os.Rename(backup, exe) // try to restore
		return fmt.Errorf("replacing binary: %w", err)
	}

	_ = os.Remove(backup)
	return nil
}

// CheckNotice returns a user-visible notice if a newer version is available.
// Returns "" on error or if the current version is up to date.
func CheckNotice(currentVersion string) string {
	if !ShouldCheck() {
		return ""
	}
	rel, err := LatestRelease(CheckTimeout)
	if err != nil {
		return ""
	}
	_ = RecordCheck() // best-effort; don't block notice on write failure
	if IsNewer(currentVersion, rel.TagName) {
		return fmt.Sprintf("\nA new version of phctl is available: %s (current: %s)\nRun 'phctl update' to upgrade.\n", rel.TagName, currentVersion)
	}
	return ""
}

func StartBackgroundCheck(currentVersion string) error {
	if !ShouldCheck() {
		return nil
	}

	exe, err := execPathFunc()
	if err != nil {
		return err
	}

	cmd := execCommand(exe, backgroundCmd, currentVersion)
	cmd.Env = append(os.Environ(), "PHCTL_NO_UPDATE_CHECK=1")
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func execPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolving executable path: %w", err)
	}
	return exe, nil
}
