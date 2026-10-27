package tiime

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAuth0 simulates the Universal Login MFA pages used by Tiime.
func fakeAuth0(
	t *testing.T,
	startOnCodePage bool,
) *httptest.Server {
	t.Helper()

	var challenge string
	mux := http.NewServeMux()

	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		var params map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&params))

		switch params["grant_type"] {
		case passwordGrant:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"mfa_required"}`))
		case "authorization_code":
			assert.Equal(t, "auth-code", params["code"])
			assert.Equal(t, redirectURI, params["redirect_uri"])
			sum := sha256.Sum256([]byte(params["code_verifier"]))
			assert.Equal(t, challenge, base64.RawURLEncoding.EncodeToString(sum[:]))
			_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":86400}`))
		default:
			t.Errorf("unexpected grant %s", params["grant_type"])
		}
	})

	mux.HandleFunc("/co/authenticate", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, webOrigin, r.Header.Get("Origin"))
		http.SetCookie(w, &http.Cookie{Name: "auth0", Value: "session", Path: "/"})
		_, _ = w.Write([]byte(`{"login_ticket":"ticket"}`))
	})

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "ticket", r.URL.Query().Get("login_ticket"))
		assert.Equal(t, "S256", r.URL.Query().Get("code_challenge_method"))
		challenge = r.URL.Query().Get("code_challenge")

		if startOnCodePage {
			http.Redirect(w, r, "/u/mfa-sms-challenge?state=s1", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/u/mfa-login-options?state=s1", http.StatusFound)
	})

	mux.HandleFunc("/u/mfa-sms-challenge", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "pick-authenticator", r.PostForm.Get("action"))
			http.Redirect(w, r, "/u/mfa-login-options?state=s1", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`<form method="POST"><input type="hidden" name="state" value="s1">
			<p>Code sent by SMS</p><input type="text" name="code">
			<button type="submit" name="action" value="pick-authenticator">Try another method</button></form>`))
	})

	mux.HandleFunc("/u/mfa-login-options", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "email::1", r.PostForm.Get("action"))
			http.Redirect(w, r, "/u/mfa-email-challenge?state=s1", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`<form method="POST"><input type="hidden" name="state" value="s1">
			<button type="submit" name="action" value="sms::0"><span>SMS</span></button>
			<button type="submit" name="action" value="email::1"><span>Email</span></button></form>`))
	})

	mux.HandleFunc("/u/mfa-email-challenge", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("auth0")
		require.NoError(t, err)
		assert.Equal(t, "session", cookie.Value)

		if r.Method == http.MethodPost {
			require.NoError(t, r.ParseForm())
			if r.PostForm.Get("code") != "123456" {
				_, _ = w.Write([]byte(`<form method="POST"><input type="hidden" name="state" value="s1">
					<p>Invalid code</p><input name="code"></form>`))
				return
			}
			http.Redirect(w, r, "/authorize/resume?state=s1", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`<form method="POST"><input type="hidden" name="state" value="s1">
			<p>Code sent by email</p><input type="text" name="code"></form>`))
	})

	mux.HandleFunc("/authorize/resume", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectURI+"?code=auth-code&state=x", http.StatusFound)
	})

	return httptest.NewServer(mux)
}

func TestAuthenticatorLogin(
	t *testing.T,
) {
	testCases := []struct {
		name            string
		startOnCodePage bool
		method          string
		codes           []string
		wantMethod      string
		wantMessages    []string
		wantErr         string
	}{
		{
			name:         "selects email by default then accepts the code",
			codes:        []string{"123456"},
			wantMethod:   "email",
			wantMessages: []string{"Code sent by email"},
		},
		{
			name:            "switches from SMS to email",
			startOnCodePage: true,
			codes:           []string{"123456"},
			wantMethod:      "email",
			wantMessages:    []string{"Code sent by email"},
		},
		{
			name:         "returns a new challenge on a wrong code",
			codes:        []string{"000000", "123456"},
			wantMethod:   "email",
			wantMessages: []string{"Code sent by email", "Invalid code"},
		},
		{
			name:            "keeps the requested SMS method",
			startOnCodePage: true,
			method:          "sms",
			wantMethod:      "sms",
			wantMessages:    []string{"Code sent by SMS"},
		},
		{
			name:    "reports unavailable methods",
			method:  "otp",
			wantErr: "choose one of: sms, email",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := fakeAuth0(t, testCase.startOnCodePage)
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

			challenge, err := auth.StartLogin(ctx, testCase.method)
			if testCase.wantErr != "" {
				require.ErrorContains(t, err, testCase.wantErr)
				return
			}
			require.NoError(t, err)

			var messages []string
			for _, code := range testCase.codes {
				require.NotNil(t, challenge)
				assert.Equal(t, testCase.wantMethod, challenge.Method)
				messages = append(messages, challenge.Message)
				challenge, err = auth.CompleteLogin(ctx, code)
				require.NoError(t, err)
			}

			if len(testCase.codes) == 0 {
				require.NotNil(t, challenge)
				assert.Equal(t, testCase.wantMethod, challenge.Method)
				assert.Contains(t, challenge.Message, testCase.wantMessages[0])
				return
			}

			assert.Nil(t, challenge)
			require.Len(t, messages, len(testCase.wantMessages))
			for i, want := range testCase.wantMessages {
				assert.Contains(t, messages[i], want)
			}

			// A new authenticator reads the cached tokens without logging in again.
			cached := NewAuthenticator(AuthConfig{AuthURL: server.URL, CachePath: cachePath, HTTPClient: server.Client()})
			token, err := cached.AccessToken(ctx)
			require.NoError(t, err)
			assert.Equal(t, "access", token)
		})
	}
}

func TestCompleteLoginWithoutPendingLogin(
	t *testing.T,
) {
	auth := NewAuthenticator(AuthConfig{})

	_, err := auth.CompleteLogin(context.Background(), "123456")

	require.ErrorIs(t, err, ErrNoPendingLogin)
}

func TestPageText(
	t *testing.T,
) {
	text := pageText(`<html><style>p{}</style><script>var a=1</script><p>Code&nbsp;sent  to <b>v***@gmail.com</b></p></html>`)

	assert.True(t, strings.HasPrefix(text, "Code"))
	assert.Contains(t, text, "v***@gmail.com")
	assert.NotContains(t, text, "var a")
}
