package tiime

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	// webOrigin and redirectURI are the ones registered for the web application.
	webOrigin   = "https://apps.tiime.fr"
	redirectURI = webOrigin + "/auth-callback"

	// DefaultMFAMethod is the verification method chosen when none is requested.
	DefaultMFAMethod = "email"

	maxLoginSteps = 20
)

// ErrNoPendingLogin is returned when a verification code is submitted without
// a login in progress.
var ErrNoPendingLogin = errors.New("no Tiime login in progress: start a login first")

// LoginChallenge describes the verification code Tiime is waiting for.
type LoginChallenge struct {
	// Method is the verification method in use, e.g. "email", "sms" or "otp".
	Method string `json:"method"`
	// Message is the text of the Tiime page, e.g. where the code was sent.
	Message string `json:"message"`
	// SentAt is when the code was requested, to look for the matching email.
	SentAt time.Time `json:"sent_at"`
}

// StartLogin logs in with the configured credentials. It returns nil when no
// verification is needed. Otherwise it returns the challenge to answer with
// CompleteLogin. method selects the MFA method, DefaultMFAMethod when empty.
func (a *Authenticator) StartLogin(
	ctx context.Context,
	method string,
) (*LoginChallenge, error) {
	if a.cfg.Email == "" || a.cfg.Password == "" {
		return nil, ErrMissingCredentials
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.pending = nil

	err := a.grant(ctx, map[string]string{
		"grant_type": passwordGrant,
		"client_id":  a.cfg.ClientID,
		"username":   a.cfg.Email,
		"password":   a.cfg.Password,
		"scope":      scope,
		"audience":   audience,
		"realm":      realm,
	})
	if !errors.Is(err, ErrMFARequired) {
		return nil, err
	}

	if method == "" {
		method = DefaultMFAMethod
	}
	flow, err := newLoginFlow(a.cfg, method, a.now)
	if err != nil {
		return nil, err
	}

	if err := flow.start(ctx); err != nil {
		return nil, err
	}

	return a.continueLogin(ctx, flow)
}

// CompleteLogin submits the verification code of the pending login. It returns
// a new challenge when Tiime rejects the code, and nil once logged in.
func (a *Authenticator) CompleteLogin(
	ctx context.Context,
	code string,
) (*LoginChallenge, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	flow := a.pending
	if flow == nil {
		return nil, ErrNoPendingLogin
	}

	if err := flow.submitCode(ctx, code); err != nil {
		a.pending = nil
		return nil, err
	}

	return a.continueLogin(ctx, flow)
}

// continueLogin advances the flow until the next challenge, or exchanges the
// authorization code for tokens.
func (a *Authenticator) continueLogin(
	ctx context.Context,
	flow *loginFlow,
) (*LoginChallenge, error) {
	authCode, challenge, err := flow.advance(ctx)
	if err != nil {
		a.pending = nil
		return nil, err
	}

	if challenge != nil {
		a.pending = flow
		return challenge, nil
	}

	a.pending = nil

	return nil, a.grant(ctx, map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     a.cfg.ClientID,
		"code":          authCode,
		"redirect_uri":  redirectURI,
		"code_verifier": flow.verifier,
	})
}

// loginFlow drives the Auth0 Universal Login pages with a cookie aware client
// that does not follow redirects, so each step can be inspected.
type loginFlow struct {
	cfg      AuthConfig
	client   *http.Client
	verifier string
	method   string
	now      func() time.Time
	page     loginPage
	switched bool
}

type loginPage struct {
	url    string
	status int
	header http.Header
	body   string
}

func newLoginFlow(
	cfg AuthConfig,
	method string,
	now func() time.Time,
) (*loginFlow, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	verifier, err := randomToken(32)
	if err != nil {
		return nil, err
	}

	return &loginFlow{
		cfg: cfg,
		client: &http.Client{
			Transport: cfg.HTTPClient.Transport,
			Timeout:   cfg.HTTPClient.Timeout,
			Jar:       jar,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		verifier: verifier,
		method:   method,
		now:      now,
	}, nil
}

// start checks the credentials and opens the authorization page.
func (f *loginFlow) start(
	ctx context.Context,
) error {
	ticket, err := f.authenticate(ctx)
	if err != nil {
		return err
	}

	authorizeURL, err := f.authorizeURL(ticket)
	if err != nil {
		return err
	}

	f.page, err = f.get(ctx, authorizeURL)

	return err
}

// advance follows the login pages, selecting the requested verification
// method, until a code is expected or the authorization code is delivered.
func (f *loginFlow) advance(
	ctx context.Context,
) (string, *LoginChallenge, error) {
	for range maxLoginSteps {
		page := f.page

		if location := page.header.Get("Location"); location != "" {
			target, err := resolve(page.url, location)
			if err != nil {
				return "", nil, err
			}
			if strings.HasPrefix(target, redirectURI) {
				code, err := authorizationCode(target)
				return code, nil, err
			}
			if f.page, err = f.get(ctx, target); err != nil {
				return "", nil, err
			}
			continue
		}

		state, err := pageState(page)
		if err != nil {
			return "", nil, err
		}
		choices := actionChoices(page.body)
		methods := methodChoices(choices)

		if codeInput.MatchString(page.body) {
			current := challengeMethod(page.url)
			canSwitch := slices.ContainsFunc(choices, func(c choice) bool { return c.value == "pick-authenticator" })
			if current != f.method && canSwitch && !f.switched {
				f.switched = true
				if f.page, err = f.postForm(ctx, page.url, url.Values{"state": {state}, "action": {"pick-authenticator"}}); err != nil {
					return "", nil, err
				}
				continue
			}

			return "", &LoginChallenge{
				Method:  current,
				Message: truncate(pageText(page.body), 300),
				SentAt:  f.now(),
			}, nil
		}

		if len(methods) == 0 {
			return "", nil, fmt.Errorf("unsupported Tiime login page (%d): %s", page.status, truncate(pageText(page.body), 300))
		}

		action, ok := pickMethod(methods, f.method)
		if !ok {
			return "", nil, fmt.Errorf(
				"verification method %q is not available, choose one of: %s",
				f.method,
				strings.Join(methodNames(methods), ", "),
			)
		}
		if f.page, err = f.postForm(ctx, page.url, url.Values{"state": {state}, "action": {action}}); err != nil {
			return "", nil, err
		}
	}

	return "", nil, errors.New("tiime login did not complete: too many steps")
}

// submitCode posts the verification code on the current challenge page.
func (f *loginFlow) submitCode(
	ctx context.Context,
	code string,
) error {
	state, err := pageState(f.page)
	if err != nil {
		return err
	}

	fields := url.Values{
		"state":  {state},
		"code":   {strings.TrimSpace(code)},
		"action": {"default"},
	}
	f.page, err = f.postForm(ctx, f.page.url, fields)

	return err
}

// authenticate checks the credentials and returns a one time login ticket.
func (f *loginFlow) authenticate(
	ctx context.Context,
) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"client_id":       f.cfg.ClientID,
		"username":        f.cfg.Email,
		"password":        f.cfg.Password,
		"realm":           realm,
		"credential_type": passwordGrant,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.cfg.AuthURL+"/co/authenticate", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", webOrigin)

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf(
			"calling Tiime authentication: %w",
			err,
		)
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		LoginTicket      string `json:"login_ticket"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.LoginTicket == "" {
		return "", fmt.Errorf(
			"tiime login refused (%d): %s %s",
			resp.StatusCode,
			body.Error,
			body.ErrorDescription,
		)
	}

	return body.LoginTicket, nil
}

func (f *loginFlow) authorizeURL(
	ticket string,
) (string, error) {
	state, err := randomToken(16)
	if err != nil {
		return "", err
	}
	nonce, err := randomToken(16)
	if err != nil {
		return "", err
	}

	challenge := sha256.Sum256([]byte(f.verifier))
	query := url.Values{
		"client_id":             {f.cfg.ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {scope},
		"audience":              {audience},
		"realm":                 {realm},
		"state":                 {state},
		"nonce":                 {nonce},
		"login_ticket":          {ticket},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}

	return f.cfg.AuthURL + "/authorize?" + query.Encode(), nil
}

var (
	stateInput   = regexp.MustCompile(`name="state"[^>]*value="([^"]+)"|value="([^"]+)"[^>]*name="state"`)
	codeInput    = regexp.MustCompile(`<input[^>]+name="code"`)
	actionButton = regexp.MustCompile(`(?s)<button[^>]*name="action"[^>]*value="([^"]+)"[^>]*>(.*?)</button>`)
	challengeURL = regexp.MustCompile(`/u/mfa-([a-z-]+?)-challenge`)
	scriptOrTag  = regexp.MustCompile(`(?s)<(script|style)[^>]*>.*?</(script|style)>|<[^>]+>`)
	spaces       = regexp.MustCompile(`\s+`)
)

type choice struct {
	value string
	label string
}

func pageState(
	page loginPage,
) (string, error) {
	match := stateInput.FindStringSubmatch(page.body)
	if match == nil {
		return "", fmt.Errorf("unexpected Tiime login page (%d): %s", page.status, truncate(pageText(page.body), 300))
	}

	return html.UnescapeString(match[1] + match[2]), nil
}

// actionChoices lists the action buttons of a page in display order.
func actionChoices(
	body string,
) []choice {
	var choices []choice
	for _, match := range actionButton.FindAllStringSubmatch(body, -1) {
		label := pageText(match[2])
		if label == "" {
			label = match[1]
		}
		choices = append(choices, choice{value: html.UnescapeString(match[1]), label: label})
	}

	return choices
}

// methodChoices keeps the verification methods, identified as
// "<method>::<index>", e.g. "email::1".
func methodChoices(
	choices []choice,
) []choice {
	return slices.DeleteFunc(slices.Clone(choices), func(c choice) bool { return !strings.Contains(c.value, "::") })
}

func pickMethod(
	methods []choice,
	method string,
) (string, bool) {
	for _, candidate := range methods {
		if strings.HasPrefix(candidate.value, method+"::") {
			return candidate.value, true
		}
	}

	return "", false
}

func methodNames(
	methods []choice,
) []string {
	names := make([]string, 0, len(methods))
	for _, method := range methods {
		names = append(names, strings.SplitN(method.value, "::", 2)[0])
	}

	return names
}

// challengeMethod reads the method from a challenge page URL such as
// /u/mfa-email-challenge.
func challengeMethod(
	pageURL string,
) string {
	match := challengeURL.FindStringSubmatch(pageURL)
	if match == nil {
		return ""
	}

	return match[1]
}

func (f *loginFlow) get(
	ctx context.Context,
	target string,
) (loginPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return loginPage{}, err
	}
	req.Header.Set("Accept", "text/html")

	return f.fetch(req)
}

func (f *loginFlow) postForm(
	ctx context.Context,
	target string,
	fields url.Values,
) (loginPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(fields.Encode()))
	if err != nil {
		return loginPage{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Auth0 answers 406 to form posts that do not accept HTML.
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Origin", f.cfg.AuthURL)
	req.Header.Set("Referer", target)

	return f.fetch(req)
}

func (f *loginFlow) fetch(
	req *http.Request,
) (loginPage, error) {
	resp, err := f.client.Do(req)
	if err != nil {
		return loginPage{}, fmt.Errorf(
			"calling Tiime login: %w",
			err,
		)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return loginPage{}, err
	}

	return loginPage{
		url:    req.URL.String(),
		status: resp.StatusCode,
		header: resp.Header,
		body:   string(body),
	}, nil
}

func authorizationCode(
	callback string,
) (string, error) {
	parsed, err := url.Parse(callback)
	if err != nil {
		return "", err
	}

	query := parsed.Query()
	if code := query.Get("code"); code != "" {
		return code, nil
	}

	return "", fmt.Errorf(
		"tiime login failed: %s %s",
		query.Get("error"),
		query.Get("error_description"),
	)
}

func resolve(
	base string,
	location string,
) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	target, err := baseURL.Parse(location)
	if err != nil {
		return "", err
	}

	return target.String(), nil
}

func pageText(
	body string,
) string {
	text := scriptOrTag.ReplaceAllString(body, " ")

	return strings.TrimSpace(spaces.ReplaceAllString(html.UnescapeString(text), " "))
}

func truncate(
	text string,
	size int,
) string {
	runes := []rune(text)
	if len(runes) <= size {
		return text
	}

	return string(runes[:size]) + "…"
}

func randomToken(
	size int,
) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
