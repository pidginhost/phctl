package cmdutil

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pidginhost "github.com/pidginhost/sdk-go"
)

func TestAPIErrorNil(t *testing.T) {
	if err := APIError("op", nil); err != nil {
		t.Errorf("APIError(_, nil) = %v, want nil", err)
	}
}

func TestAPIErrorPlainError(t *testing.T) {
	err := APIError("listing things", errors.New("connection refused"))
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	got := err.Error()
	want := "listing things: connection refused"
	if got != want {
		t.Errorf("APIError plain = %q, want %q", got, want)
	}
}

func TestAPIErrorSDKErrorIncludesResponseBodyAndKeepsChain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ipv4": ["Invalid pk \"99\""]}`))
	}))
	defer server.Close()

	apiClient := pidginhost.New("test-token", server.URL)
	_, _, err := apiClient.CloudAPI.CloudIpv4Create(context.Background()).Execute()
	if err == nil {
		t.Fatal("expected SDK error")
	}

	wrapped := APIError("creating IPv4", err)
	if wrapped == nil {
		t.Fatal("expected wrapped error")
	}
	if got := wrapped.Error(); !strings.Contains(got, `creating IPv4: 400 Bad Request: ipv4=Invalid pk "99"`) {
		t.Fatalf("wrapped error = %q", got)
	}
	var sdkErr *pidginhost.GenericOpenAPIError
	if !errors.As(wrapped, &sdkErr) {
		t.Fatalf("wrapped error does not preserve GenericOpenAPIError chain: %v", wrapped)
	}
}

// TestFormatAPIBodyJSONObject pins field order. Ranging a Go map made the same
// 400 render differently between runs, which is unreadable in a terminal and
// impossible to assert on in a script.
func TestFormatAPIBodyJSONObject(t *testing.T) {
	body := []byte(`{"non_field_errors": ["must be active"], "ipv4": ["Invalid pk \"99\""]}`)
	want := `ipv4=Invalid pk "99"; non_field_errors=must be active`
	for i := 0; i < 20; i++ {
		if got := formatAPIBody(body); got != want {
			t.Fatalf("formatAPIBody = %q, want %q (run %d)", got, want, i)
		}
	}
}

// TestFormatAPIBodyValidationErrors covers the 400 shapes the API returns for
// the server attach routes, which previously came back as 200-with-message or
// as "Object with address=None does not exist".
func TestFormatAPIBodyValidationErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing required field",
			body: `{"ipv4": ["This field is required."]}`,
			want: "ipv4=This field is required.",
		},
		{
			name: "invalid reboot value",
			body: `{"reboot": ["Must be a valid boolean."]}`,
			want: "reboot=Must be a valid boolean.",
		},
		{
			name: "multiple messages on one field",
			body: `{"ipv6": ["Server already has a different IPv6.", "Detach it first."]}`,
			want: "ipv6=Server already has a different IPv6.; ipv6=Detach it first.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatAPIBody([]byte(tc.body)); got != tc.want {
				t.Errorf("formatAPIBody = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatAPIBodyJSONDetail(t *testing.T) {
	body := []byte(`{"detail": "Authentication credentials were not provided."}`)
	got := formatAPIBody(body)
	want := "detail=Authentication credentials were not provided."
	if got != want {
		t.Errorf("formatAPIBody detail = %q, want %q", got, want)
	}
}

func TestFormatAPIBodyPlainText(t *testing.T) {
	body := []byte("Bad Request\n")
	got := formatAPIBody(body)
	want := "Bad Request"
	if got != want {
		t.Errorf("formatAPIBody plain = %q, want %q", got, want)
	}
}

func TestFormatAPIBodyJSONStringArray(t *testing.T) {
	body := []byte(`["IP is not attached.", "extra detail"]`)
	got := formatAPIBody(body)
	for _, want := range []string{"IP is not attached.", "extra detail"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatAPIBody array missing %q in %q", want, got)
		}
	}
}
