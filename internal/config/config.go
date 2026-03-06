package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AuthToken string `yaml:"auth_token"`
	APIURL    string `yaml:"api_url"`
	Output    string `yaml:"output"`
}

func DefaultConfig() *Config {
	return &Config{
		APIURL: "https://www.pidginhost.com",
		Output: "table",
	}
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
		_ = yaml.Unmarshal(data, saveCfg)
	}

	// Override with the values the caller set
	if cfg.AuthToken != "" {
		saveCfg.AuthToken = cfg.AuthToken
	}
	if cfg.APIURL != "" && cfg.APIURL != "https://www.pidginhost.com" {
		saveCfg.APIURL = cfg.APIURL
	}
	if cfg.Output != "" && cfg.Output != "table" {
		saveCfg.Output = cfg.Output
	}

	out, err := yaml.Marshal(saveCfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, out, 0600)
}
