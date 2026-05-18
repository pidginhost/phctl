package cmdutil

import (
	"errors"
	"strings"
	"testing"
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

func TestFormatAPIBodyJSONObject(t *testing.T) {
	body := []byte(`{"ipv4": ["Invalid pk \"99\""], "non_field_errors": ["must be active"]}`)
	got := formatAPIBody(body)
	// Map iteration order isn't deterministic; check substring presence.
	for _, want := range []string{`ipv4=Invalid pk "99"`, "non_field_errors=must be active"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatAPIBody missing %q in %q", want, got)
		}
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
