package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	pidginhost "github.com/pidginhost/sdk-go"

	"github.com/pidginhost/phctl/internal/config"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

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
func RawGet(path string, dst interface{}) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.AuthToken == "" {
		return fmt.Errorf("no API token configured. Run 'phctl auth init' or set PIDGINHOST_API_TOKEN")
	}
	url := strings.TrimRight(cfg.APIURL, "/") + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AuthToken)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, path)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// RawFetchAll paginates through a DRF list endpoint using raw HTTP,
// bypassing SDK type mismatches (e.g. decimal strings vs float64).
func RawFetchAll[T any](path string) ([]T, error) {
	var all []T
	for page := int32(1); ; page++ {
		pagePath := fmt.Sprintf("%s?page=%d", path, page)
		var resp PaginatedResponse[T]
		if err := RawGet(pagePath, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Results...)
		if resp.Next == nil {
			break
		}
	}
	return all, nil
}
