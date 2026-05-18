package cmdutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	pidginhost "github.com/pidginhost/sdk-go"
)

// APIError wraps an SDK error so the caller's prefix is preserved, the HTTP
// status line stays visible, and the response body (if any) is surfaced as a
// readable single-line message instead of being silently dropped.
//
// Usage: `return cmdutil.APIError("attaching IPv4", err)`.
func APIError(op string, err error) error {
	if err == nil {
		return nil
	}
	var apiErr pidginhost.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		body := apiErr.Body()
		if len(body) > 0 {
			if msg := formatAPIBody(body); msg != "" {
				return fmt.Errorf("%s: %w: %s", op, err, msg)
			}
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

func formatAPIBody(body []byte) string {
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return strings.TrimSpace(string(body))
	}
	parts := flattenAPIBody("", parsed)
	if len(parts) == 0 {
		return strings.TrimSpace(string(body))
	}
	return strings.Join(parts, "; ")
}

func flattenAPIBody(prefix string, v any) []string {
	switch t := v.(type) {
	case map[string]any:
		var out []string
		for k, val := range t {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			out = append(out, flattenAPIBody(key, val)...)
		}
		return out
	case []any:
		var out []string
		for _, item := range t {
			out = append(out, flattenAPIBody(prefix, item)...)
		}
		return out
	case string:
		if prefix == "" {
			return []string{t}
		}
		return []string{prefix + "=" + t}
	case nil:
		return nil
	default:
		s := fmt.Sprintf("%v", t)
		if prefix == "" {
			return []string{s}
		}
		return []string{prefix + "=" + s}
	}
}
