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

const (
	fakeAccessKey = "AKIAEXAMPLEACCESSKEY"
	fakeSecretKey = "s3cr3tEXAMPLEsecretkeyvalue"
)

// credentialsServer answers the bucket credentials route with a 200 whose body
// carries the keys but cannot decode into BucketCredentials, which is the case
// that puts a live secret inside an error value.
func credentialsServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// access_key as a number: a 200 the model cannot decode.
		_, _ = w.Write([]byte(`{"bucket":"c1-assets","endpoint":"https://s3.example",` +
			`"region":"eu-1","access_key":12345,"secret_key":"` + fakeSecretKey + `"}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func revealError(t *testing.T, url string) error {
	t.Helper()
	apiClient := pidginhost.New("test-token", url)
	_, _, err := apiClient.CloudAPI.CloudBucketsCredentialsRevealCreate(context.Background(), 7).Execute()
	if err == nil {
		t.Fatal("expected SDK error")
	}
	return err
}

func TestAPIErrorLeaksTheCredentialBody(t *testing.T) {
	// Pins why APIErrorRedacted has to exist: the general wrapper is correct
	// everywhere else precisely because it surfaces the body.
	err := revealError(t, credentialsServer(t).URL)

	if got := APIError("revealing credentials", err).Error(); !strings.Contains(got, fakeSecretKey) {
		t.Fatalf("expected APIError to surface the body; got %q", got)
	}
}

func TestAPIErrorRedactedNeverSurfacesTheSecret(t *testing.T) {
	err := revealError(t, credentialsServer(t).URL)

	wrapped := APIErrorRedacted("revealing credentials", err)
	if wrapped == nil {
		t.Fatal("expected wrapped error")
	}
	got := wrapped.Error()

	for _, secret := range []string{fakeSecretKey, fakeAccessKey, "12345"} {
		if strings.Contains(got, secret) {
			t.Errorf("redacted error leaked %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "revealing credentials") {
		t.Errorf("redacted error lost the operation prefix: %q", got)
	}
	if !strings.Contains(got, "withheld") {
		t.Errorf("redacted error should say a body was withheld, got %q", got)
	}
	var sdkErr *pidginhost.GenericOpenAPIError
	if !errors.As(wrapped, &sdkErr) {
		t.Errorf("redacted error dropped the GenericOpenAPIError chain: %v", wrapped)
	}
}

func TestAPIErrorRedactedNilAndPlainErrors(t *testing.T) {
	if APIErrorRedacted("op", nil) != nil {
		t.Error("nil error should stay nil")
	}
	got := APIErrorRedacted("op", errors.New("dial tcp: refused")).Error()
	if got != "op: dial tcp: refused" {
		t.Errorf("plain error = %q, want %q", got, "op: dial tcp: refused")
	}
	if strings.Contains(got, "withheld") {
		t.Errorf("no body to withhold, should not claim one: %q", got)
	}
}
