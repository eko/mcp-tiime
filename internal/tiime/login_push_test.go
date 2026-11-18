package tiime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pushPage = `<form method="POST"><input type="hidden" name="state" value="s1">
	<p>Nous avons envoyé une notification à l'appareil suivant via l'application Tiime</p>
	<input type="checkbox" name="rememberBrowser">
	<button type="submit" name="action" value="default">J'ai répondu sur mon appareil</button></form>
	<form method="POST"><input type="hidden" name="state" value="s1">
	<button type="submit" name="action" value="resend">Renvoyer</button></form>`

// fakePushAuth0 simulates a Tiime account whose MFA is a push notification.
// The notification counts as approved once approved is set.
func fakePushAuth0(
	t *testing.T,
	approved *atomic.Bool,
) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		var params map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&params))
		if params["grant_type"] == passwordGrant {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"mfa_required"}`))
			return
		}
		assert.Equal(t, "auth-code", params["code"])
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":86400}`))
	})
	mux.HandleFunc("/co/authenticate", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"login_ticket":"ticket"}`))
	})
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/u/mfa-push-challenge-push?state=s1", http.StatusFound)
	})
	mux.HandleFunc("/u/mfa-push-challenge-push", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "default", r.PostForm.Get("action"))
			assert.Empty(t, r.PostForm.Get("code"))
			if approved.Load() {
				http.Redirect(w, r, "/authorize/resume?state=s1", http.StatusFound)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
		}
		_, _ = w.Write([]byte(pushPage))
	})
	mux.HandleFunc("/authorize/resume", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, ClientApp.redirectURI()+"?code=auth-code&state=x", http.StatusFound)
	})

	return httptest.NewServer(mux)
}

func TestPushLogin(
	t *testing.T,
) {
	var approved atomic.Bool
	server := fakePushAuth0(t, &approved)
	defer server.Close()

	cachePath := filepath.Join(t.TempDir(), "token.json")
	auth := NewAuthenticator(AuthConfig{
		AuthURL:    server.URL,
		ClientID:   "client",
		Email:      "me@example.com",
		Password:   "secret",
		CachePath:  cachePath,
		HTTPClient: server.Client(),
	})
	ctx := context.Background()

	// Without a method, a push account stays on the push page.
	challenge, err := auth.StartLogin(ctx, "")
	require.NoError(t, err)
	require.NotNil(t, challenge)
	assert.Equal(t, PushMFAMethod, challenge.Method)
	assert.Contains(t, challenge.Message, "notification")

	// Confirming before the approval shows the push page again.
	challenge, err = auth.CompleteLogin(ctx, "")
	require.NoError(t, err)
	require.NotNil(t, challenge)
	assert.Equal(t, PushMFAMethod, challenge.Method)

	approved.Store(true)
	challenge, err = auth.CompleteLogin(ctx, "")
	require.NoError(t, err)
	assert.Nil(t, challenge)

	token, err := NewAuthenticator(AuthConfig{AuthURL: server.URL, CachePath: cachePath, HTTPClient: server.Client()}).AccessToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "access", token)
}

func TestPushForm(
	t *testing.T,
) {
	testCases := []struct {
		name       string
		body       string
		wantOK     bool
		wantFields map[string]string
	}{
		{
			name:       "button with name before value",
			body:       pushPage,
			wantOK:     true,
			wantFields: map[string]string{"state": "s1", "action": "default"},
		},
		{
			name: "button with value before name, resend form first",
			body: `<form method="POST"><input type="hidden" value="s1" name="state">
				<button value="resend" type="submit" name="action">Renvoyer</button></form>
				<form method="POST" data-form-primary="true"><input value="s1" type="hidden" name="state">
				<button class="c-btn" value="default" type="submit" name="action">J'ai répondu sur mon appareil</button></form>`,
			wantOK:     true,
			wantFields: map[string]string{"state": "s1", "action": "default"},
		},
		{
			name: "hidden action input and an unnamed button",
			body: `<form method="POST"><input type="hidden" name="state" value="s1">
				<input type="hidden" name="action" value="default">
				<input type="checkbox" name="rememberBrowser">
				<button type="submit">I've responded on my device</button></form>`,
			wantOK:     true,
			wantFields: map[string]string{"state": "s1", "action": "default"},
		},
		{
			name:   "only secondary forms",
			body:   `<form><input type="hidden" name="state" value="s1"><button name="action" value="resend">Renvoyer</button></form>`,
			wantOK: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fields, ok := pushForm(testCase.body)
			require.Equal(t, testCase.wantOK, ok)
			for name, want := range testCase.wantFields {
				assert.Equal(t, want, fields.Get(name), name)
			}
			assert.Empty(t, fields.Get("rememberBrowser"))
		})
	}
}

func TestChallengeMethodPush(
	t *testing.T,
) {
	assert.Equal(t, PushMFAMethod, challengeMethod("https://auth.tiime.fr/u/mfa-push-challenge-push?state=s1"))
	assert.Equal(t, "email", challengeMethod("https://auth.tiime.fr/u/mfa-email-challenge?state=s1"))
}
