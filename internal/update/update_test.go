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
	"strings"
	"testing"
	"time"
)

const testBinaryPayload = "new-binary"

func testBinarySum() string {
	h := sha256.Sum256([]byte(testBinaryPayload))
	return hex.EncodeToString(h[:])
}

// makeReleaseHandler returns a roundTripFunc that serves a binary asset and
// (optionally) a checksums.txt asset. checksumsBody is served verbatim; pass
// an empty string to skip the checksums.txt asset URL entirely.
func makeReleaseHandler(t *testing.T, binaryURL, payload, checksumsURL, checksumsBody string) roundTripFunc {
	t.Helper()
	return func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case binaryURL:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(payload)),
				Header:     make(http.Header),
			}, nil
		case checksumsURL:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(checksumsBody)),
				Header:     make(http.Header),
			}, nil
		default:
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
				Header:     make(http.Header),
			}, nil
		}
	}
}

func withLatestReleaseURL(t *testing.T, url string) {
	t.Helper()
	old := latestReleaseURL
	latestReleaseURL = url
	t.Cleanup(func() {
		latestReleaseURL = old
	})
}

func withExecPathFunc(t *testing.T, fn func() (string, error)) {
	t.Helper()
	old := execPathFunc
	execPathFunc = fn
	t.Cleanup(func() {
		execPathFunc = old
	})
}

func withExecCommand(t *testing.T, fn func(name string, arg ...string) *exec.Cmd) {
	t.Helper()
	old := execCommand
	execCommand = fn
	t.Cleanup(func() {
		execCommand = old
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func withHTTPClient(t *testing.T, fn roundTripFunc) {
	t.Helper()
	old := newHTTPClient
	newHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout:   timeout,
			Transport: fn,
		}
	}
	t.Cleanup(func() {
		newHTTPClient = old
	})
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	outPath := os.Getenv("PHCTL_TEST_HELPER_OUTPUT")
	lines := append([]string{}, os.Args[1:]...)
	lines = append(lines, "PHCTL_NO_UPDATE_CHECK="+os.Getenv("PHCTL_NO_UPDATE_CHECK"))
	if err := os.WriteFile(outPath, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"1.0.0", "1.0.1", true},
		{"v1.0.0", "v1.0.1", true},
		{"1.0.0", "1.1.0", true},
		{"1.0.0", "2.0.0", true},
		{"1.2.3", "1.2.3", false},
		{"1.2.4", "1.2.3", false},
		{"2.0.0", "1.9.9", false},
		{"dev", "1.0.0", false},
		{"1.0.0", "dev", false},
		{"1.0.0-rc1", "1.0.0", false},
		{"1.0.0", "1.0.1-rc1", true},
	}

	for _, tt := range tests {
		if got := IsNewer(tt.current, tt.latest); got != tt.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestIsDevBuild(t *testing.T) {
	cases := map[string]bool{
		"":        true,
		"dev":     true,
		"garbage": true,
		"1.2":     true,
		"1.2.3":   false,
		"v1.2.3":  false,
		"v0.6.1":  false,
	}
	for v, want := range cases {
		if got := IsDevBuild(v); got != want {
			t.Errorf("IsDevBuild(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1.2.3", []int{1, 2, 3}},
		{"v1.2.3", []int{1, 2, 3}},
		{"1.0.0-rc1", []int{1, 0, 0}},
		{"dev", nil},
		{"1.2", nil},
		{"abc.def.ghi", nil},
	}

	for _, tt := range tests {
		got := parseVersion(tt.input)
		if tt.want == nil {
			if got != nil {
				t.Errorf("parseVersion(%q) = %v, want nil", tt.input, got)
			}
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("parseVersion(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseVersion(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestAssetName(t *testing.T) {
	want := "phctl-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		want += ".exe"
	}

	if got := assetName(); got != want {
		t.Errorf("assetName() = %q, want %q", got, want)
	}
}

func TestSupportsSelfUpdate(t *testing.T) {
	if !supportsSelfUpdate("linux") {
		t.Fatal("supportsSelfUpdate(linux) = false, want true")
	}
	if supportsSelfUpdate("windows") {
		t.Fatal("supportsSelfUpdate(windows) = true, want false")
	}
}

func TestShouldCheck(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if !ShouldCheck() {
		t.Fatal("ShouldCheck() = false with no file, want true")
	}

	if err := RecordCheck(); err != nil {
		t.Fatalf("RecordCheck() error: %v", err)
	}
	if ShouldCheck() {
		t.Fatal("ShouldCheck() = true right after RecordCheck, want false")
	}

	path, err := lastCheckPath()
	if err != nil {
		t.Fatalf("lastCheckPath() error: %v", err)
	}
	if err := os.WriteFile(path, []byte("0"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if !ShouldCheck() {
		t.Fatal("ShouldCheck() = false with old timestamp, want true")
	}
}

func TestLatestRelease(t *testing.T) {
	rel := Release{
		TagName: "v1.2.3",
		Assets: []Asset{
			{Name: "phctl-linux-amd64", BrowserDownloadURL: "https://example.com/phctl-linux-amd64"},
		},
	}
	withLatestReleaseURL(t, "https://updates.example.test/releases/latest")
	withHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != latestReleaseURL {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
				Header:     make(http.Header),
			}, nil
		}

		data, err := json.Marshal(rel)
		if err != nil {
			t.Fatalf("json.Marshal() error: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data))),
			Header:     make(http.Header),
		}, nil
	})

	got, err := LatestRelease(2 * time.Second)
	if err != nil {
		t.Fatalf("LatestRelease() error: %v", err)
	}
	if got.TagName != rel.TagName {
		t.Fatalf("TagName = %q, want %q", got.TagName, rel.TagName)
	}
	if len(got.Assets) != 1 || got.Assets[0].Name != rel.Assets[0].Name {
		t.Fatalf("Assets = %+v, want %+v", got.Assets, rel.Assets)
	}
}

func TestLatestReleaseSendsUserAgent(t *testing.T) {
	withLatestReleaseURL(t, "https://updates.example.test/releases/latest")
	var gotUA string
	withHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		gotUA = r.Header.Get("User-Agent")
		data, _ := json.Marshal(Release{TagName: "v0.0.0"})
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data))),
			Header:     make(http.Header),
		}, nil
	})

	if _, err := LatestRelease(time.Second); err != nil {
		t.Fatalf("LatestRelease error: %v", err)
	}
	if !strings.HasPrefix(gotUA, "phctl/") {
		t.Errorf("User-Agent = %q, want phctl/<version> prefix", gotUA)
	}
}

func TestDownloadAssetSendsUserAgent(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "phctl")
	if err := os.WriteFile(exe, []byte("current"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	withExecPathFunc(t, func() (string, error) { return exe, nil })

	checksums := fmt.Sprintf("%s  %s\n", testBinarySum(), assetName())
	var binaryUA string
	withHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://example.test/asset":
			binaryUA = r.Header.Get("User-Agent")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(testBinaryPayload)),
				Header:     make(http.Header),
			}, nil
		case "https://example.test/checksums.txt":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(checksums)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})

	rel := &Release{
		TagName: "v1.2.3",
		Assets: []Asset{
			{Name: assetName(), BrowserDownloadURL: "https://example.test/asset"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.test/checksums.txt"},
		},
	}
	if _, err := DownloadAsset(rel); err != nil {
		t.Fatalf("DownloadAsset error: %v", err)
	}
	if !strings.HasPrefix(binaryUA, "phctl/") {
		t.Errorf("User-Agent on binary download = %q, want phctl/<version> prefix", binaryUA)
	}
}

func TestDownloadAsset(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "phctl")
	if err := os.WriteFile(exe, []byte("current-binary"), 0755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	withExecPathFunc(t, func() (string, error) {
		return exe, nil
	})
	checksums := fmt.Sprintf("%s  %s\n", testBinarySum(), assetName())
	withHTTPClient(t, makeReleaseHandler(t, "https://downloads.example.test/download", testBinaryPayload, "https://downloads.example.test/checksums.txt", checksums))

	path, err := DownloadAsset(&Release{
		TagName: "v1.2.3",
		Assets: []Asset{
			{Name: assetName(), BrowserDownloadURL: "https://downloads.example.test/download"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://downloads.example.test/checksums.txt"},
		},
	})
	if err != nil {
		t.Fatalf("DownloadAsset() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != testBinaryPayload {
		t.Fatalf("downloaded binary = %q, want %q", string(data), testBinaryPayload)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("DownloadAsset() path dir = %q, want %q", filepath.Dir(path), dir)
	}
}

func TestDownloadAsset_RejectsWithoutChecksumsAsset(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "phctl")
	if err := os.WriteFile(exe, []byte("current"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	withExecPathFunc(t, func() (string, error) { return exe, nil })
	withHTTPClient(t, makeReleaseHandler(t, "https://example.test/binary", testBinaryPayload, "", ""))

	rel := &Release{
		TagName: "v1.2.3",
		Assets: []Asset{
			{Name: assetName(), BrowserDownloadURL: "https://example.test/binary"},
		},
	}
	_, err := DownloadAsset(rel)
	if err == nil {
		t.Fatal("expected error when release has no checksums.txt asset")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error = %q, want it to mention 'checksum'", err)
	}
}

func TestDownloadAsset_RejectsOnChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "phctl")
	if err := os.WriteFile(exe, []byte("current"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	withExecPathFunc(t, func() (string, error) { return exe, nil })

	wrongSum := strings.Repeat("00", sha256.Size)
	checksumsBody := fmt.Sprintf("%s  %s\n", wrongSum, assetName())
	withHTTPClient(t, makeReleaseHandler(t, "https://example.test/binary", testBinaryPayload, "https://example.test/checksums.txt", checksumsBody))

	rel := &Release{
		TagName: "v1.2.3",
		Assets: []Asset{
			{Name: assetName(), BrowserDownloadURL: "https://example.test/binary"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.test/checksums.txt"},
		},
	}
	_, err := DownloadAsset(rel)
	if err == nil {
		t.Fatal("expected error on checksum mismatch")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "checksum") {
		t.Errorf("error = %q, want it to mention 'checksum'", err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "phctl" {
			t.Errorf("leftover file after mismatch: %s", e.Name())
		}
	}
}

func TestDownloadAsset_RejectsWhenEntryMissing(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "phctl")
	if err := os.WriteFile(exe, []byte("current"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	withExecPathFunc(t, func() (string, error) { return exe, nil })

	checksumsBody := "deadbeef  some-other-asset\n"
	withHTTPClient(t, makeReleaseHandler(t, "https://example.test/binary", testBinaryPayload, "https://example.test/checksums.txt", checksumsBody))

	rel := &Release{
		TagName: "v1.2.3",
		Assets: []Asset{
			{Name: assetName(), BrowserDownloadURL: "https://example.test/binary"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.test/checksums.txt"},
		},
	}
	_, err := DownloadAsset(rel)
	if err == nil {
		t.Fatal("expected error when asset is not listed in checksums.txt")
	}
}

func TestDownloadAsset_AcceptsValidChecksum(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "phctl")
	if err := os.WriteFile(exe, []byte("current"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	withExecPathFunc(t, func() (string, error) { return exe, nil })

	checksumsBody := fmt.Sprintf("%s  %s\nffff  some-other\n", testBinarySum(), assetName())
	withHTTPClient(t, makeReleaseHandler(t, "https://example.test/binary", testBinaryPayload, "https://example.test/checksums.txt", checksumsBody))

	rel := &Release{
		TagName: "v1.2.3",
		Assets: []Asset{
			{Name: assetName(), BrowserDownloadURL: "https://example.test/binary"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.test/checksums.txt"},
		},
	}
	path, err := DownloadAsset(rel)
	if err != nil {
		t.Fatalf("DownloadAsset error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != testBinaryPayload {
		t.Errorf("payload = %q, want %q", got, testBinaryPayload)
	}
}

func TestApply(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "phctl")
	tmp := filepath.Join(dir, "phctl-update-123")

	if err := os.WriteFile(exe, []byte("old-binary"), 0755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.WriteFile(tmp, []byte("new-binary"), 0755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	withExecPathFunc(t, func() (string, error) {
		return exe, nil
	})

	err := Apply(tmp)
	if runtime.GOOS == "windows" {
		if !errors.Is(err, ErrSelfUpdateUnsupported) {
			t.Fatalf("Apply() error = %v, want %v", err, ErrSelfUpdateUnsupported)
		}
		return
	}
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "new-binary" {
		t.Fatalf("binary content = %q, want %q", string(data), "new-binary")
	}
	if _, err := os.Stat(exe + ".bak"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup file should be removed, stat error = %v", err)
	}
}

func TestCheckNoticeFailureDoesNotThrottle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	withLatestReleaseURL(t, "https://updates.example.test/releases/latest")
	withHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("boom")),
			Header:     make(http.Header),
		}, nil
	})

	if notice := CheckNotice("1.0.0"); notice != "" {
		t.Fatalf("CheckNotice() = %q, want empty string", notice)
	}
	if !ShouldCheck() {
		t.Fatal("ShouldCheck() = false after failed check, want true")
	}
}

func TestCheckNoticeSuccessRecordsCheck(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	withLatestReleaseURL(t, "https://updates.example.test/releases/latest")
	withHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		data, err := json.Marshal(Release{
			TagName: "v1.1.0",
			Assets:  []Asset{},
		})
		if err != nil {
			t.Fatalf("json.Marshal() error: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data))),
			Header:     make(http.Header),
		}, nil
	})

	notice := CheckNotice("1.0.0")
	if !strings.Contains(notice, "v1.1.0") {
		t.Fatalf("CheckNotice() = %q, want notice to mention v1.1.0", notice)
	}
	if ShouldCheck() {
		t.Fatal("ShouldCheck() = true after successful check, want false")
	}
}

func TestStartBackgroundCheck(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	outPath := filepath.Join(tmp, "helper-output.txt")
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("PHCTL_TEST_HELPER_OUTPUT", outPath)

	withExecPathFunc(t, func() (string, error) {
		return "/fake/phctl", nil
	})
	withExecCommand(t, func(name string, arg ...string) *exec.Cmd {
		args := append([]string{"-test.run=TestHelperProcess", "--", name}, arg...)
		return exec.Command(os.Args[0], args...)
	})

	if err := StartBackgroundCheck("1.2.3"); err != nil {
		t.Fatalf("StartBackgroundCheck() error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		data, err := os.ReadFile(outPath)
		if err == nil {
			content := string(data)
			if !strings.Contains(content, "/fake/phctl") {
				t.Fatalf("helper args = %q, want executable path", content)
			}
			if !strings.Contains(content, backgroundCmd) {
				t.Fatalf("helper args = %q, want %q", content, backgroundCmd)
			}
			if !strings.Contains(content, "1.2.3") {
				t.Fatalf("helper args = %q, want current version", content)
			}
			if !strings.Contains(content, "PHCTL_NO_UPDATE_CHECK=1") {
				t.Fatalf("helper env = %q, want PHCTL_NO_UPDATE_CHECK=1", content)
			}
			return
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("ReadFile() error: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for helper process output")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
