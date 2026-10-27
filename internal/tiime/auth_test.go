package tiime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticatorAccessToken(
	t *testing.T,
) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name        string
		cached      *storedToken
		responses   map[string]string
		statuses    map[string]int
		wantToken   string
		wantGrants  []string
		wantErr     error
		wantRefresh string
	}{
		{
			name:        "uses valid cached token",
			cached:      &storedToken{AccessToken: "cached", RefreshToken: "r1", ExpiresAt: now.Add(time.Hour)},
			wantToken:   "cached",
			wantRefresh: "r1",
		},
		{
			name:   "refreshes expired token and keeps refresh token",
			cached: &storedToken{AccessToken: "old", RefreshToken: "r1", ExpiresAt: now.Add(-time.Minute)},
			responses: map[string]string{
				"refresh_token": `{"access_token":"refreshed","expires_in":3600}`,
			},
			wantToken:   "refreshed",
			wantGrants:  []string{"refresh_token"},
			wantRefresh: "r1",
		},
		{
			name:   "falls back to password when refresh fails",
			cached: &storedToken{AccessToken: "old", RefreshToken: "revoked", ExpiresAt: now.Add(-time.Minute)},
			responses: map[string]string{
				"refresh_token": `{"error":"invalid_grant"}`,
				passwordGrant:   `{"access_token":"fresh","refresh_token":"r2","expires_in":3600}`,
			},
			statuses:    map[string]int{"refresh_token": http.StatusForbidden},
			wantToken:   "fresh",
			wantGrants:  []string{"refresh_token", passwordGrant},
			wantRefresh: "r2",
		},
		{
			name: "logs in without cache",
			responses: map[string]string{
				passwordGrant: `{"access_token":"fresh","refresh_token":"r2","expires_in":3600}`,
			},
			wantToken:   "fresh",
			wantGrants:  []string{passwordGrant},
			wantRefresh: "r2",
		},
		{
			name: "reports MFA",
			responses: map[string]string{
				passwordGrant: `{"error":"mfa_required","error_description":"Multifactor authentication required"}`,
			},
			statuses:   map[string]int{passwordGrant: http.StatusForbidden},
			wantGrants: []string{passwordGrant},
			wantErr:    ErrMFARequired,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var grants []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/oauth/token", r.URL.Path)

				var params map[string]string
				require.NoError(t, json.NewDecoder(r.Body).Decode(&params))
				grant := params["grant_type"]
				grants = append(grants, grant)

				if status, ok := testCase.statuses[grant]; ok {
					w.WriteHeader(status)
				}
				_, _ = w.Write([]byte(testCase.responses[grant]))
			}))
			defer server.Close()

			cachePath := filepath.Join(t.TempDir(), "token.json")
			if testCase.cached != nil {
				content, err := json.Marshal(testCase.cached)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(cachePath, content, 0o600))
			}

			auth := NewAuthenticator(AuthConfig{
				AuthURL:    server.URL,
				ClientID:   "client",
				Email:      "me@example.com",
				Password:   "secret",
				CachePath:  cachePath,
				HTTPClient: server.Client(),
			})
			auth.now = func() time.Time { return now }

			token, err := auth.AccessToken(context.Background())

			assert.Equal(t, testCase.wantGrants, grants)
			if testCase.wantErr != nil {
				require.ErrorIs(t, err, testCase.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.wantToken, token)

			content, err := os.ReadFile(cachePath)
			require.NoError(t, err)
			var saved storedToken
			require.NoError(t, json.Unmarshal(content, &saved))
			assert.Equal(t, testCase.wantRefresh, saved.RefreshToken)

			info, err := os.Stat(cachePath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		})
	}
}
