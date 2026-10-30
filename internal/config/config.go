// Package config loads the server configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"mcp-tiime/internal/tiime"
)

// Modes select the Tiime web application the server acts as.
const (
	// ModeClient drives a company account, as apps.tiime.fr does.
	ModeClient = "client"
	// ModeExpert drives an accounting firm account, as expert.tiime.fr does.
	ModeExpert = "expert"
)

// Config holds the server settings, all read from TIIME_* environment variables.
type Config struct {
	Mode       string `mapstructure:"mode"`
	Email      string `mapstructure:"email"`
	Password   string `mapstructure:"password"`
	CompanyID  int64  `mapstructure:"company_id"`
	TokenCache string `mapstructure:"token_cache"`
	PDFDir     string `mapstructure:"pdf_dir"`
	APIURL     string `mapstructure:"api_url"`
	AuthURL    string `mapstructure:"auth_url"`
	ClientID   string `mapstructure:"client_id"`
	AppVersion string `mapstructure:"app_version"`
	LogLevel   string `mapstructure:"log_level"`
}

// Load reads the configuration from the environment and applies defaults.
func Load() (Config, error) {
	v := viper.New()
	v.SetEnvPrefix("tiime")
	v.AutomaticEnv()

	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf(
			"resolving home directory: %w",
			err,
		)
	}

	// client_id, app_version and token_cache depend on the mode: they are
	// filled after decoding unless set explicitly.
	defaults := map[string]any{
		"mode":        ModeClient,
		"email":       "",
		"password":    "",
		"company_id":  0,
		"token_cache": "",
		"pdf_dir":     filepath.Join(home, "Downloads"),
		"api_url":     "https://chronos-api.tiime-apps.com/v1",
		"auth_url":    "https://auth0.tiime.fr",
		"client_id":   "",
		"app_version": "",
		"log_level":   "info",
	}
	for key, value := range defaults {
		v.SetDefault(key, value)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf(
			"decoding configuration: %w",
			err,
		)
	}

	app, err := cfg.App()
	if err != nil {
		return Config{}, err
	}
	if cfg.ClientID == "" {
		cfg.ClientID = app.ClientID
	}
	if cfg.AppVersion == "" {
		cfg.AppVersion = app.Version
	}
	if cfg.TokenCache == "" {
		// One session per mode: the client and expert applications use
		// different OAuth clients, so their tokens are not interchangeable.
		name := "token.json"
		if cfg.Mode == ModeExpert {
			name = "token-expert.json"
		}
		cfg.TokenCache = filepath.Join(home, ".config", "mcp-tiime", name)
	}

	return cfg, nil
}

// App returns the Tiime web application matching the mode.
func (c Config) App() (tiime.App, error) {
	switch c.Mode {
	case ModeClient, "":
		return tiime.ClientApp, nil
	case ModeExpert:
		return tiime.ExpertApp, nil
	default:
		return tiime.App{}, fmt.Errorf(
			"unknown TIIME_MODE %q, expected %s or %s",
			c.Mode,
			ModeClient,
			ModeExpert,
		)
	}
}
