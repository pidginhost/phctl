package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	pidginhost "github.com/pidginhost/sdk-go"

	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/config"
)

var httpClient = &http.Client{Timeout: cmdutil.DefaultAPITimeout}

// PaginatedResponse is the generic DRF paginated response wrapper.
type PaginatedResponse[T any] struct {
	Count    int     `json:"count"`
	Next     *string `json:"next"`
	Previous *string `json:"previous"`
	Results  []T     `json:"results"`
}

func New() (*pidginhost.APIClient, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if cfg.AuthToken == "" {
		return nil, fmt.Errorf("no API token configured. Run 'phctl auth init' or set PIDGINHOST_API_TOKEN")
	}
	return pidginhost.New(cfg.AuthToken, cfg.APIURL), nil
}

// RawGet makes an authenticated GET request and decodes JSON into dst.
// Use this to bypass SDK type mismatches (e.g. decimal strings vs float64).
func RawGet(ctx context.Context, path string, dst interface{}) error {
	return rawRequest(ctx, http.MethodGet, path, nil, dst, http.StatusOK)
}

func RawPost(ctx context.Context, path string, body interface{}, dst interface{}, expectedStatuses ...int) error {
	if len(expectedStatuses) == 0 {
		expectedStatuses = []int{http.StatusOK}
	}
	return rawRequest(ctx, http.MethodPost, path, body, dst, expectedStatuses...)
}

func RawDelete(ctx context.Context, path string) error {
	return rawRequest(ctx, http.MethodDelete, path, nil, nil, http.StatusNoContent)
}

func rawRequest(ctx context.Context, method string, path string, body interface{}, dst interface{}, expectedStatuses ...int) error {
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.AuthToken == "" {
		return fmt.Errorf("no API token configured. Run 'phctl auth init' or set PIDGINHOST_API_TOKEN")
	}
	url := strings.TrimRight(cfg.APIURL, "/") + path
	var reader io.Reader
	if body != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
		reader = &buf
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+cfg.AuthToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if !expectedStatus(resp.StatusCode, expectedStatuses) {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if len(body) > 0 {
			return fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, path, body)
		}
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, path)
	}
	if dst == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func expectedStatus(got int, expected []int) bool {
	for _, want := range expected {
		if got == want {
			return true
		}
	}
	return false
}

// RawFetchAll paginates through a DRF list endpoint using raw HTTP,
// bypassing SDK type mismatches (e.g. decimal strings vs float64).
func RawFetchAll[T any](ctx context.Context, path string) ([]T, error) {
	var all []T
	for page := int32(1); page <= cmdutil.MaxPages; page++ {
		pagePath := fmt.Sprintf("%s?page=%d", path, page)
		var resp PaginatedResponse[T]
		if err := RawGet(ctx, pagePath, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Results...)
		if resp.Next == nil {
			return all, nil
		}
	}
	return nil, fmt.Errorf("pagination exceeded %d pages for %s; aborting to avoid runaway loop", cmdutil.MaxPages, path)
}
