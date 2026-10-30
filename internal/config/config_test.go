package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcp-tiime/internal/tiime"
)

func TestLoadMode(
	t *testing.T,
) {
	home := t.TempDir()

	testCases := []struct {
		name           string
		env            map[string]string
		wantMode       string
		wantClientID   string
		wantAppVersion string
		wantTokenCache string
		wantErr        string
	}{
		{
			name:           "defaults to the client application",
			wantMode:       ModeClient,
			wantClientID:   tiime.ClientApp.ClientID,
			wantAppVersion: tiime.ClientApp.Version,
			wantTokenCache: filepath.Join(home, ".config", "mcp-tiime", "token.json"),
		},
		{
			name:           "expert mode uses the expert application and its own session",
			env:            map[string]string{"TIIME_MODE": "expert"},
			wantMode:       ModeExpert,
			wantClientID:   tiime.ExpertApp.ClientID,
			wantAppVersion: tiime.ExpertApp.Version,
			wantTokenCache: filepath.Join(home, ".config", "mcp-tiime", "token-expert.json"),
		},
		{
			name: "explicit values win over the mode defaults",
			env: map[string]string{
				"TIIME_MODE":        "expert",
				"TIIME_CLIENT_ID":   "custom",
				"TIIME_APP_VERSION": "9.9.9",
				"TIIME_TOKEN_CACHE": "/tmp/session.json",
			},
			wantMode:       ModeExpert,
			wantClientID:   "custom",
			wantAppVersion: "9.9.9",
			wantTokenCache: "/tmp/session.json",
		},
		{
			name:    "rejects an unknown mode",
			env:     map[string]string{"TIIME_MODE": "admin"},
			wantErr: `unknown TIIME_MODE "admin"`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			for _, key := range []string{"TIIME_MODE", "TIIME_CLIENT_ID", "TIIME_APP_VERSION", "TIIME_TOKEN_CACHE"} {
				t.Setenv(key, "")
			}
			for key, value := range testCase.env {
				t.Setenv(key, value)
			}

			cfg, err := Load()
			if testCase.wantErr != "" {
				require.ErrorContains(t, err, testCase.wantErr)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, testCase.wantMode, cfg.Mode)
			assert.Equal(t, testCase.wantClientID, cfg.ClientID)
			assert.Equal(t, testCase.wantAppVersion, cfg.AppVersion)
			assert.Equal(t, testCase.wantTokenCache, cfg.TokenCache)
		})
	}
}
