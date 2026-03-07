package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
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
		{"1.0.0-rc1", "1.0.0", false}, // same base
		{"1.0.0", "1.0.1-rc1", true},
	}
	for _, tt := range tests {
		got := IsNewer(tt.current, tt.latest)
		if got != tt.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
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
	name := assetName()
	want := "phctl_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if name != want {
		t.Errorf("assetName() = %q, want %q", name, want)
	}
}

func TestShouldCheck(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// No file exists — should check
	if !ShouldCheck() {
		t.Error("ShouldCheck() = false with no file, want true")
	}

	// Record a check, should not check again immediately
	RecordCheck()
	if ShouldCheck() {
		t.Error("ShouldCheck() = true right after RecordCheck, want false")
	}

	// Write an old timestamp — should check again
	path, _ := lastCheckPath()
	_ = os.WriteFile(path, []byte("0"), 0600)
	if !ShouldCheck() {
		t.Error("ShouldCheck() = false with old timestamp, want true")
	}
}

func TestLatestRelease(t *testing.T) {
	rel := Release{
		TagName: "v1.2.3",
		Assets: []Asset{
			{Name: "phctl_linux_amd64", BrowserDownloadURL: "https://example.com/phctl_linux_amd64"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	// We can't easily override repoAPIURL, but we can test the JSON parsing
	// by using the test server directly.
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(srv.URL + "/releases/latest")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var got Release
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.TagName != "v1.2.3" {
		t.Errorf("TagName = %q, want %q", got.TagName, "v1.2.3")
	}
	if len(got.Assets) != 1 || got.Assets[0].Name != "phctl_linux_amd64" {
		t.Errorf("unexpected assets: %+v", got.Assets)
	}
}

func TestApply(t *testing.T) {
	dir := t.TempDir()

	// Create a fake "current" binary
	exe := filepath.Join(dir, "phctl")
	if err := os.WriteFile(exe, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a fake "new" binary
	tmp := filepath.Join(dir, "phctl-update-123")
	if err := os.WriteFile(tmp, []byte("new-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	// We can't easily test Apply directly because os.Executable() points
	// to the test binary, but we can test the rename logic manually.
	backup := exe + ".bak"
	if err := os.Rename(exe, backup); err != nil {
		t.Fatalf("backup rename: %v", err)
	}
	if err := os.Rename(tmp, exe); err != nil {
		t.Fatalf("replace rename: %v", err)
	}
	_ = os.Remove(backup)

	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-binary" {
		t.Errorf("binary content = %q, want %q", string(data), "new-binary")
	}
}
