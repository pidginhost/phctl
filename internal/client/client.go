package client

import (
	"fmt"

	pidginhost "github.com/pidginhost/sdk-go"

	"github.com/pidginhost/phctl/internal/config"
)

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
