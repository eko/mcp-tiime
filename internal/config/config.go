// Package config loads the server configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the server settings, all read from TIIME_* environment variables.
type Config struct {
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

	defaults := map[string]any{
		"email":       "",
		"password":    "",
		"company_id":  0,
		"token_cache": filepath.Join(home, ".config", "mcp-tiime", "token.json"),
		"pdf_dir":     filepath.Join(home, "Downloads"),
		"api_url":     "https://chronos-api.tiime-apps.com/v1",
		"auth_url":    "https://auth0.tiime.fr",
		// Public OAuth client ID of the apps.tiime.fr web application.
		"client_id":   "iEbsbe3o66gcTBfGRa012kj1Rb6vjAND",
		"app_version": "4.40.6",
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

	return cfg, nil
}
