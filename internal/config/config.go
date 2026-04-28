package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/pidginhost/phctl/internal/cmdutil"
)

// writeAtomic is a seam so tests can simulate atomic-write failures.
var writeAtomic = cmdutil.WriteAtomic

const DefaultAPIURL = "https://www.pidginhost.com"

type Config struct {
	AuthToken   string `yaml:"auth_token"`
	APIURL      string `yaml:"api_url"`
	Output      string `yaml:"output"`
	UpdateCheck *bool  `yaml:"update_check,omitempty"`
}

func DefaultConfig() *Config {
	t := true
	return &Config{
		APIURL:      DefaultAPIURL,
		Output:      "table",
		UpdateCheck: &t,
	}
}

// UpdateCheckEnabled returns whether automatic update checks are enabled.
// Defaults to true. Disabled by setting update_check: false in config
// or PHCTL_NO_UPDATE_CHECK=1 in the environment.
func (c *Config) UpdateCheckEnabled() bool {
	if os.Getenv("PHCTL_NO_UPDATE_CHECK") == "1" {
		return false
	}
	if c.UpdateCheck != nil {
		return *c.UpdateCheck
	}
	return true
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "phctl"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Load file first
	path, err := Path()
	if err != nil {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		fileCfg := &Config{}
		if err := yaml.Unmarshal(data, fileCfg); err != nil {
			return nil, fmt.Errorf("parsing config: %w", err)
		}
		if fileCfg.AuthToken != "" {
			cfg.AuthToken = fileCfg.AuthToken
		}
		if fileCfg.APIURL != "" {
			cfg.APIURL = fileCfg.APIURL
		}
		if fileCfg.Output != "" {
			cfg.Output = fileCfg.Output
		}
		if fileCfg.UpdateCheck != nil {
			cfg.UpdateCheck = fileCfg.UpdateCheck
		}
	}

	// Environment variables override file values
	if token := os.Getenv("PIDGINHOST_API_TOKEN"); token != "" {
		cfg.AuthToken = token
	}
	if url := os.Getenv("PIDGINHOST_API_URL"); url != "" {
		cfg.APIURL = url
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path, err := Path()
	if err != nil {
		return err
	}

	// Only save token and API URL that were explicitly set by the user.
	// Do not persist env-var-sourced values.
	saveCfg := &Config{}

	// Load the existing file to preserve user-set values
	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, saveCfg); err != nil {
			return fmt.Errorf("parsing existing config: %w", err)
		}
	}

	// Override with the values the caller set
	if cfg.AuthToken != "" {
		saveCfg.AuthToken = cfg.AuthToken
	}
	if cfg.APIURL != "" && cfg.APIURL != DefaultAPIURL {
		saveCfg.APIURL = cfg.APIURL
	}
	if cfg.Output != "" && cfg.Output != "table" {
		saveCfg.Output = cfg.Output
	}
	if cfg.UpdateCheck != nil {
		saveCfg.UpdateCheck = cfg.UpdateCheck
	}

	out, err := yaml.Marshal(saveCfg)
	if err != nil {
		return err
	}

	return writeAtomic(path, out, 0600)
}
