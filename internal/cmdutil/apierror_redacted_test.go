package cmdutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	pidginhost "github.com/pidginhost/sdk-go"
)

const (
	fakeAccessKey = "example-access-key-not-a-credential"
	fakeSecretKey = "example-secret-key-not-a-credential"
)

type credentialRoundTripFunc func(*http.Request) (*http.Response, error)

func (f credentialRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// revealError answers the bucket credentials route with a caller-chosen body
// so both decode errors and non-2xx responses exercise redaction.
func revealError(t *testing.T, status int, payload string) error {
	return revealErrorWithStatusLine(t, status,
		fmt.Sprintf("%d %s", status, http.StatusText(status)), payload)
}

func revealErrorWithStatusLine(t *testing.T, status int, statusLine, payload string) error {
	t.Helper()
	cfg := pidginhost.NewConfiguration()
	cfg.HTTPClient = &http.Client{Transport: credentialRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Status:     statusLine,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(payload)),
			Request:    r,
		}, nil
	})}
	apiClient := pidginhost.NewAPIClient(cfg)
	_, _, err := apiClient.CloudAPI.CloudBucketsCredentialsRevealCreate(context.Background(), 7).Execute()
	if err == nil {
		t.Fatal("expected SDK error")
	}
	return err
}

func TestAPIErrorLeaksTheCredentialBody(t *testing.T) {
	// Pins why APIErrorRedacted has to exist: the general wrapper is correct
	// everywhere else precisely because it surfaces the body.
	payload := `{"bucket":"c1-assets","endpoint":"https://s3.example",` +
		`"region":"eu-1","access_key":12345,"secret_key":"` + fakeSecretKey + `"}`
	err := revealError(t, http.StatusOK, payload)

	if got := APIError("revealing credentials", err).Error(); !strings.Contains(got, fakeSecretKey) {
		t.Fatalf("expected APIError to surface the body; got %q", got)
	}
}

func TestAPIErrorRedactedNeverSurfacesTheSecret(t *testing.T) {
	payload := `{"bucket":"c1-assets","endpoint":"https://s3.example",` +
		`"region":"eu-1","access_key":12345,"secret_key":"` + fakeSecretKey + `"}`
	err := revealError(t, http.StatusOK, payload)

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
	if errors.As(wrapped, &sdkErr) {
		t.Error("redacted error still exposes the credential body through errors.As")
	}
}

func TestAPIErrorRedactedSanitizesNestedAndNon2xxErrors(t *testing.T) {
	payload := `{"access_key":"` + fakeAccessKey + `","secret_key":"` + fakeSecretKey + `"}`

	for _, status := range []int{http.StatusOK, http.StatusBadRequest} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			err := revealError(t, status, payload)
			// APIError has already copied the response body into this outer error.
			// Redaction must use the underlying SDK status, not the leaky wrapper.
			wrapped := APIErrorRedacted("credential operation", APIError("request failed", err))
			got := wrapped.Error()
			for _, secret := range []string{fakeAccessKey, fakeSecretKey} {
				if strings.Contains(got, secret) {
					t.Errorf("redacted error leaked %q: %s", secret, got)
				}
			}
			var sdkErr *pidginhost.GenericOpenAPIError
			if errors.As(wrapped, &sdkErr) {
				t.Error("redacted error still exposes its withheld body")
			}
		})
	}
}

func TestAPIErrorRedactedDoesNotTrustTheHTTPReasonPhrase(t *testing.T) {
	err := revealErrorWithStatusLine(t, http.StatusBadRequest,
		"400 "+fakeSecretKey, "")

	wrapped := APIErrorRedacted("credential operation", err)
	if strings.Contains(wrapped.Error(), fakeSecretKey) {
		t.Fatalf("redacted error leaked the reason phrase: %s", wrapped)
	}
	var sdkErr *pidginhost.GenericOpenAPIError
	if errors.As(wrapped, &sdkErr) {
		t.Error("redacted error still exposes the untrusted SDK error")
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
