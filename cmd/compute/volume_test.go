package compute

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const createdVolume = `{"id":9,"alias":"data","size":20,"product":"fast-storage","attached":true,` +
	`"server":"web1","status":"pending","billing":null}`

// setVolumeCreateFlags sets flags on the shared command and restores them, so
// one test's --alias cannot leak into the next request body.
func setVolumeCreateFlags(t *testing.T, flags map[string]string) {
	t.Helper()
	for name, value := range flags {
		f := volumeCreateCmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("volume create has no --%s flag", name)
		}
		prev := f.Value.String()
		if err := f.Value.Set(value); err != nil {
			t.Fatalf("--%s=%s: %v", name, value, err)
		}
		t.Cleanup(func() { _ = f.Value.Set(prev) })
	}
}

func TestVolumeHasCreateAndProducts(t *testing.T) {
	have := map[string]bool{}
	for _, c := range volumeCmd.Commands() {
		have[c.Name()] = true
	}
	for _, want := range []string{"create", "products"} {
		if !have[want] {
			t.Errorf("compute volume is missing subcommand %q", want)
		}
	}
}

func TestVolumeCreateRequiresServerSizeAndProduct(t *testing.T) {
	for _, name := range []string{"server", "size", "product"} {
		f := volumeCreateCmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("volume create has no --%s flag", name)
		}
		if f.Annotations[cobra.BashCompOneRequiredFlag] == nil {
			t.Errorf("--%s must be required", name)
		}
	}
}

func TestVolumeCreateSendsTheVolume(t *testing.T) {
	setVolumeCreateFlags(t, map[string]string{"server": "42", "size": "20", "product": "fast-storage", "alias": "data"})

	st, out, err := runOp(t, volumeCreateCmd, nil, okOp(createdVolume), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodPost || st.path != "/api/cloud/servers/42/volumes/" {
		t.Errorf("request = %s %s, want POST /api/cloud/servers/42/volumes/", st.method, st.path)
	}
	assertBodyKeys(t, st.body, "product", "size", "alias")
	if st.body["product"] != "fast-storage" || st.body["size"] != float64(20) || st.body["alias"] != "data" {
		t.Errorf("body = %v", st.body)
	}
	if out != "Volume 9 created on server 42 (20 GB).\n" {
		t.Errorf("output = %q", out)
	}
}

func TestVolumeCreateHonoursOutputFlag(t *testing.T) {
	setVolumeCreateFlags(t, map[string]string{"server": "42", "size": "20", "product": "fast-storage"})

	_, out, err := runOp(t, volumeCreateCmd, nil, okOp(createdVolume), "json", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil || got["id"] != float64(9) {
		t.Errorf("json output = %q (err %v), want the created volume", out, err)
	}
}

func TestVolumeCreateRejectsANullBody(t *testing.T) {
	setVolumeCreateFlags(t, map[string]string{"server": "42", "size": "20", "product": "fast-storage"})

	if _, _, err := runOp(t, volumeCreateCmd, nil, okOp(`null`), "table", "", false); err == nil {
		t.Fatal("a null API response must return an error")
	}
}

func TestVolumeCreateKeepsTheAPIReason(t *testing.T) {
	setVolumeCreateFlags(t, map[string]string{"server": "42", "size": "5", "product": "fast-storage"})

	reject := func(*opStub) (int, string) {
		return http.StatusBadRequest, `{"size":["Volume has invalid size."]}`
	}
	_, _, err := runOp(t, volumeCreateCmd, nil, reject, "table", "", false)
	if err == nil || !strings.Contains(err.Error(), "Volume has invalid size.") {
		t.Errorf("error = %v, want the API's reason", err)
	}
}

func TestVolumeProductsListsTheSlugs(t *testing.T) {
	page := `{"count":1,"next":null,"previous":null,"results":[{"id":11,"slug":"fast-storage",` +
		`"name":"SSD Storage","type":"storage","unit":"GB","price":"0.1000","min_size":10,"max_size":1000}]}`

	st, out, err := runOp(t, volumeProductsCmd, nil, okOp(page), "table", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodGet || st.path != "/api/cloud/storage-products/" {
		t.Errorf("request = %s %s, want GET /api/cloud/storage-products/", st.method, st.path)
	}
	for _, want := range []string{"SLUG", "fast-storage", "SSD Storage", "0.1000", "10", "1000"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output %q is missing %q", out, want)
		}
	}

	_, out, err = runOp(t, volumeProductsCmd, nil, okOp(page), "json", "", false)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got) != 1 || got[0]["slug"] != "fast-storage" {
		t.Errorf("json output = %q (err %v), want one product", out, err)
	}
}
