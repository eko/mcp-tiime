package tiime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	audience = "https://chronos/"
	realm    = "Chronos-prod-db"
	scope    = "openid email offline_access"

	passwordGrant = "http://auth0.com/oauth/grant-type/password-realm"

	// expiryMargin renews the access token slightly before Auth0 rejects it.
	expiryMargin = time.Minute
)

// ErrMFARequired is returned when the Tiime account has two-factor authentication
// enabled and no refresh token is cached, as the password grant cannot complete it.
var ErrMFARequired = errors.New("the Tiime account requires MFA: log in with the start_login tool or `mcp-tiime login`")

// ErrLoginRequired is returned when no session is cached for an application
// that only accepts the interactive login flow.
var ErrLoginRequired = errors.New("no valid Tiime session: log in with the start_login tool or `mcp-tiime login`")

// ErrMissingCredentials is returned when a login is needed but TIIME_EMAIL or
// TIIME_PASSWORD is not set.
var ErrMissingCredentials = errors.New("no valid Tiime session: run `mcp-tiime login`, or set TIIME_EMAIL and TIIME_PASSWORD on the server to log in with the start_login tool")

// AuthConfig configures the Auth0 login against Tiime.
type AuthConfig struct {
	// App is the web application to log in as, ClientApp when zero.
	App        App
	AuthURL    string
	ClientID   string
	Email      string
	Password   string
	CachePath  string
	HTTPClient *http.Client
}

// Authenticator provides Tiime access tokens, refreshing them when needed and
// persisting the refresh token so restarts do not trigger a new login.
type Authenticator struct {
	cfg   AuthConfig
	now   func() time.Time
	mu    sync.Mutex
	token *storedToken
	// pending is the MFA login waiting for its verification code.
	pending *loginFlow
}

type storedToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type authError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// NewAuthenticator creates an Authenticator.
func NewAuthenticator(
	cfg AuthConfig,
) *Authenticator {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}

	return &Authenticator{
		cfg: cfg,
		now: time.Now,
	}
}

// AccessToken returns a valid access token, logging in or refreshing if needed.
func (a *Authenticator) AccessToken(
	ctx context.Context,
) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.token == nil {
		a.token = a.loadCache()
	}

	if a.token != nil && a.now().Before(a.token.ExpiresAt) {
		return a.token.AccessToken, nil
	}

	if a.token != nil && a.token.RefreshToken != "" {
		err := a.grant(ctx, map[string]string{
			"grant_type":    "refresh_token",
			"client_id":     a.cfg.ClientID,
			"refresh_token": a.token.RefreshToken,
		})
		if err == nil {
			return a.token.AccessToken, nil
		}
	}

	if a.cfg.app().LoginFlowOnly {
		return "", ErrLoginRequired
	}

	if a.cfg.Email == "" || a.cfg.Password == "" {
		return "", ErrMissingCredentials
	}

	err := a.grant(ctx, map[string]string{
		"grant_type": passwordGrant,
		"client_id":  a.cfg.ClientID,
		"username":   a.cfg.Email,
		"password":   a.cfg.Password,
		"scope":      scope,
		"audience":   audience,
		"realm":      realm,
	})
	if err != nil {
		return "", err
	}

	return a.token.AccessToken, nil
}

// Invalidate forces the next AccessToken call to renew the access token.
func (a *Authenticator) Invalidate() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.token != nil {
		a.token.ExpiresAt = time.Time{}
	}
}

func (c AuthConfig) app() App {
	if c.App.Origin == "" {
		return ClientApp
	}

	return c.App
}

func (a *Authenticator) grant(
	ctx context.Context,
	params map[string]string,
) error {
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.cfg.AuthURL+"/oauth/token",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf(
			"calling Tiime authentication: %w",
			err,
		)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var authErr authError
		_ = json.NewDecoder(resp.Body).Decode(&authErr)
		if authErr.Error == "mfa_required" {
			return ErrMFARequired
		}
		return fmt.Errorf(
			"tiime authentication failed (%s): %s %s",
			params["grant_type"],
			authErr.Error,
			authErr.ErrorDescription,
		)
	}

	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return fmt.Errorf(
			"decoding Tiime token: %w",
			err,
		)
	}

	refreshToken := tok.RefreshToken
	// Auth0 does not always rotate the refresh token: keep the previous one.
	if refreshToken == "" && a.token != nil {
		refreshToken = a.token.RefreshToken
	}

	a.token = &storedToken{
		AccessToken:  tok.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    a.now().Add(time.Duration(tok.ExpiresIn)*time.Second - expiryMargin),
	}

	return a.saveCache()
}

func (a *Authenticator) loadCache() *storedToken {
	if a.cfg.CachePath == "" {
		return nil
	}

	content, err := os.ReadFile(a.cfg.CachePath)
	if err != nil {
		return nil
	}

	var tok storedToken
	if err := json.Unmarshal(content, &tok); err != nil {
		return nil
	}

	return &tok
}

func (a *Authenticator) saveCache() error {
	if a.cfg.CachePath == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(a.cfg.CachePath), 0o700); err != nil {
		return fmt.Errorf(
			"creating token cache directory: %w",
			err,
		)
	}

	content, err := json.Marshal(a.token)
	if err != nil {
		return err
	}

	// The refresh token grants full account access: keep the file private.
	if err := os.WriteFile(a.cfg.CachePath, content, 0o600); err != nil {
		return fmt.Errorf(
			"writing token cache: %w",
			err,
		)
	}

	return nil
}
