package compute

import (
	"bytes"
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

func TestVolumeCreateConfirmsBeforeBilling(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stdin     string
		force     bool
		wantCalls int
	}{
		{name: "declined", stdin: "n\n"},
		{name: "EOF"},
		{name: "confirmed", stdin: "yes\n", wantCalls: 1},
		{name: "forced", force: true, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setVolumeCreateFlags(t, map[string]string{"server": "42", "size": "20", "product": "fast-storage"})
			st, out, err := runOp(t, volumeCreateCmd, nil, okOp(createdVolume), "json", tc.stdin, tc.force)
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			if st.calls != tc.wantCalls {
				t.Errorf("API calls = %d, want %d", st.calls, tc.wantCalls)
			}
			prompt := volumeCreateCmd.ErrOrStderr().(*bytes.Buffer).String()
			if tc.force {
				if prompt != "" {
					t.Errorf("--force prompted: %q", prompt)
				}
			} else {
				for _, want := range []string{"20 GB", "fast-storage", "server 42", "paid", "[y/N]"} {
					if !strings.Contains(prompt, want) {
						t.Errorf("confirmation %q is missing %q", prompt, want)
					}
				}
			}
			if tc.wantCalls == 0 {
				if out != "" {
					t.Errorf("cancelled create wrote stdout: %q", out)
				}
			} else {
				var got map[string]interface{}
				if err := json.Unmarshal([]byte(out), &got); err != nil || got["id"] != float64(9) {
					t.Errorf("json output = %q (err %v), want the created volume", out, err)
				}
			}
		})
	}
}

func TestVolumeCreateSendsTheVolume(t *testing.T) {
	setVolumeCreateFlags(t, map[string]string{"server": "42", "size": "20", "product": "fast-storage", "alias": "data", "project": "demo"})

	response := strings.Replace(createdVolume, `"alias":"data"`, `"alias":"data","project":"demo"`, 1)
	st, out, err := runOp(t, volumeCreateCmd, nil, okOp(response), "table", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if st.method != http.MethodPost || st.path != "/api/cloud/servers/42/volumes/" {
		t.Errorf("request = %s %s, want POST /api/cloud/servers/42/volumes/", st.method, st.path)
	}
	assertBodyKeys(t, st.body, "product", "size", "alias", "project")
	if st.body["product"] != "fast-storage" || st.body["size"] != float64(20) || st.body["alias"] != "data" || st.body["project"] != "demo" {
		t.Errorf("body = %v", st.body)
	}
	if out != "Volume 9 created on server 42 (20 GB).\n" {
		t.Errorf("output = %q", out)
	}
}

func TestVolumeCreateHonoursOutputFlag(t *testing.T) {
	setVolumeCreateFlags(t, map[string]string{"server": "42", "size": "20", "product": "fast-storage"})

	st, out, err := runOp(t, volumeCreateCmd, nil, okOp(createdVolume), "json", "", true)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil || got["id"] != float64(9) {
		t.Errorf("json output = %q (err %v), want the created volume", out, err)
	}
	assertBodyKeys(t, st.body, "product", "size")
}

func TestVolumeCreateRejectsANullBody(t *testing.T) {
	setVolumeCreateFlags(t, map[string]string{"server": "42", "size": "20", "product": "fast-storage"})

	if _, _, err := runOp(t, volumeCreateCmd, nil, okOp(`null`), "table", "", true); err == nil {
		t.Fatal("a null API response must return an error")
	}
}

func TestVolumeCreateKeepsTheAPIReason(t *testing.T) {
	setVolumeCreateFlags(t, map[string]string{"server": "42", "size": "5", "product": "fast-storage"})

	reject := func(*opStub) (int, string) {
		return http.StatusBadRequest, `{"size":["Volume has invalid size."]}`
	}
	_, _, err := runOp(t, volumeCreateCmd, nil, reject, "table", "", true)
	if err == nil || !strings.Contains(err.Error(), "Volume has invalid size.") {
		t.Errorf("error = %v, want the API's reason", err)
	}
}

func TestVolumeCreateRejectsUnappliedChanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		from string
		to   string
	}{
		{name: "attachment", from: `"attached":true`, to: `"attached":false`},
		{name: "size", from: `"size":20`, to: `"size":10`},
		{name: "alias", from: `"alias":"data"`, to: `"alias":"other"`},
		{name: "project", from: `"project":"demo"`, to: `"project":"other"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setVolumeCreateFlags(t, map[string]string{"server": "42", "size": "20", "product": "fast-storage", "alias": "data", "project": "demo"})
			response := strings.Replace(createdVolume, `"alias":"data"`, `"alias":"data","project":"demo"`, 1)
			response = strings.Replace(response, tc.from, tc.to, 1)
			st, out, err := runOp(t, volumeCreateCmd, nil, okOp(response), "json", "", true)
			if err == nil {
				t.Errorf("unapplied %s must return an error", tc.name)
			}
			if out != "" {
				t.Errorf("unapplied %s reported success: %q", tc.name, out)
			}
			if st.calls != 1 {
				t.Errorf("API calls = %d, want 1", st.calls)
			}
		})
	}
}

func TestVolumeCreateAcceptsNormalizedNames(t *testing.T) {
	for _, tc := range []struct {
		name    string
		alias   string
		project string
		want    string
	}{
		{name: "trimmed names", alias: " data ", project: " demo ", want: "data"},
		{name: "inherited project", alias: "data", project: " ", want: "data"},
		{name: "blank alias", alias: " ", project: "demo", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setVolumeCreateFlags(t, map[string]string{
				"server": "42", "size": "20", "product": "11", "alias": tc.alias, "project": tc.project,
			})
			response := strings.Replace(createdVolume, `"alias":"data"`, `"alias":"`+tc.want+`","project":"demo"`, 1)
			st, _, err := runOp(t, volumeCreateCmd, nil, okOp(response), "json", "", true)
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			if st.calls != 1 || st.body["product"] != "11" {
				t.Errorf("API calls = %d, body = %v, want one creation using product ID", st.calls, st.body)
			}
		})
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

func TestVolumeProductsPaginatesAndHandlesErrors(t *testing.T) {
	firstPage := `{"count":2,"next":"https://example.com/api/cloud/storage-products/?page=2","previous":null,` +
		`"results":[{"id":11,"slug":"fast-storage","name":"SSD Storage","type":"storage",` +
		`"unit":"GB","price":"0.1000","min_size":10,"max_size":1000}]}`
	lastPage := `{"count":2,"next":null,"previous":null,` +
		`"results":[{"id":12,"slug":"bulk-storage","name":"Bulk Storage","type":"storage",` +
		`"unit":"GB","price":"0.0500","min_size":10,"max_size":2000}]}`
	for _, tc := range []struct {
		name    string
		code    int
		body    string
		wantErr string
	}{
		{name: "all pages", code: http.StatusOK, body: lastPage},
		{name: "null page", code: http.StatusOK, body: `null`, wantErr: "server returned no response page"},
		{name: "API error", code: http.StatusForbidden, body: `{"detail":"Permission denied."}`, wantErr: "Permission denied."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			respond := func(st *opStub) (int, string) {
				if st.calls == 1 {
					if st.query != "page=1" {
						t.Errorf("first query = %q, want page=1", st.query)
					}
					return http.StatusOK, firstPage
				}
				if st.query != "page=2" {
					t.Errorf("second query = %q, want page=2", st.query)
				}
				return tc.code, tc.body
			}
			st, out, err := runOp(t, volumeProductsCmd, nil, respond, "json", "", false)
			if st.calls != 2 {
				t.Errorf("API calls = %d, want 2", st.calls)
			}
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %v, want %q", err, tc.wantErr)
				}
				if out != "" {
					t.Errorf("failed list returned partial results: %q", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("RunE: %v", err)
			}
			var got []map[string]interface{}
			if err := json.Unmarshal([]byte(out), &got); err != nil || len(got) != 2 {
				t.Fatalf("json output = %q (err %v), want both products", out, err)
			}
			if got[0]["slug"] != "fast-storage" || got[1]["slug"] != "bulk-storage" || got[1]["price"] != "0.0500" {
				t.Errorf("products = %v", got)
			}
		})
	}
}
