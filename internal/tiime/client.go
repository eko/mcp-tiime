// Package tiime is a client for the internal, undocumented Tiime API used by
// the apps.tiime.fr web application.
package tiime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const defaultLimit = 50

// ErrNotFound is returned when Tiime answers 404.
var ErrNotFound = errors.New("tiime resource not found")

// TokenProvider supplies bearer tokens for the Tiime API.
type TokenProvider interface {
	AccessToken(
		ctx context.Context,
	) (string, error)
	Invalidate()
}

// APIError is a non successful response from the Tiime API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("tiime API returned %d: %s", e.StatusCode, e.Body)
}

// Client calls the Tiime API.
type Client struct {
	baseURL    string
	appHeader  string
	appVersion string
	tokens     TokenProvider
	httpClient *http.Client
}

// ClientOption customizes a Client.
type ClientOption func(*Client)

// WithAppHeader sets the tiime-app header, which identifies the web
// application ("tiime" by default, "tiime-pro" for accounting firms).
func WithAppHeader(
	header string,
) ClientOption {
	return func(c *Client) {
		c.appHeader = header
	}
}

// NewClient creates a Tiime API client.
func NewClient(
	baseURL string,
	appVersion string,
	tokens TokenProvider,
	httpClient *http.Client,
	opts ...ClientOption,
) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	client := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		appHeader:  ClientApp.Header,
		appVersion: appVersion,
		tokens:     tokens,
		httpClient: httpClient,
	}
	for _, opt := range opts {
		opt(client)
	}

	return client
}

// Companies lists the companies the user has access to.
func (c *Client) Companies(
	ctx context.Context,
) ([]Company, error) {
	var companies []Company
	_, err := c.getJSON(ctx, "/companies", nil, nil, &companies)

	return companies, err
}

// Customers lists the invoicing clients of a company.
func (c *Client) Customers(
	ctx context.Context,
	companyID int64,
	filter CustomerFilter,
	window Range,
) (Page[Customer], error) {
	query := url.Values{}
	if filter.Archived != "" {
		query.Set("archived", filter.Archived)
	}

	return getPage[Customer](ctx, c, companyPath(companyID, "/clients"), query, nil, window, "")
}

// Customer returns the full detail of a client.
func (c *Client) Customer(
	ctx context.Context,
	companyID int64,
	customerID int64,
) (json.RawMessage, error) {
	return c.getRaw(ctx, companyPath(companyID, "/clients/"+strconv.FormatInt(customerID, 10)), nil)
}

// Contacts lists the contacts of a client.
func (c *Client) Contacts(
	ctx context.Context,
	companyID int64,
	customerID int64,
) ([]Contact, error) {
	var contacts []Contact
	path := companyPath(companyID, "/clients/"+strconv.FormatInt(customerID, 10)+"/contacts")
	_, err := c.getJSON(ctx, path, nil, nil, &contacts)

	return contacts, err
}

// Invoices lists the invoices of a company.
func (c *Client) Invoices(
	ctx context.Context,
	companyID int64,
	filter DocumentFilter,
	window Range,
) (Page[Document], error) {
	return getPage[Document](ctx, c, companyPath(companyID, "/invoices"), documentQuery(filter), nil, window, "")
}

// Invoice returns the full detail of an invoice, including its lines.
func (c *Client) Invoice(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
) (json.RawMessage, error) {
	return c.getRaw(ctx, companyPath(companyID, "/invoices/"+strconv.FormatInt(invoiceID, 10)), nil)
}

// InvoicePDF downloads the PDF of an invoice.
func (c *Client) InvoicePDF(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
) ([]byte, error) {
	query := url.Values{"id": {strconv.FormatInt(invoiceID, 10)}}
	resp, err := c.do(ctx, request{
		method:  http.MethodGet,
		path:    companyPath(companyID, "/invoices/pdf"),
		query:   query,
		headers: map[string]string{"Accept": "application/pdf"},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	return io.ReadAll(resp.Body)
}

// Quotes lists the quotes of a company.
func (c *Client) Quotes(
	ctx context.Context,
	companyID int64,
	filter DocumentFilter,
	window Range,
) (Page[Document], error) {
	return getPage[Document](ctx, c, companyPath(companyID, "/quotations"), documentQuery(filter), nil, window, "")
}

// Quote returns the full detail of a quote, including its lines.
func (c *Client) Quote(
	ctx context.Context,
	companyID int64,
	quoteID int64,
) (json.RawMessage, error) {
	return c.getRaw(ctx, companyPath(companyID, "/quotations/"+strconv.FormatInt(quoteID, 10)), nil)
}

func getPage[T any](
	ctx context.Context,
	c *Client,
	path string,
	query url.Values,
	headers map[string]string,
	window Range,
	wrapKey string,
) (Page[T], error) {
	if window.Limit <= 0 {
		window.Limit = defaultLimit
	}

	rangeHeaders := map[string]string{
		"Range": fmt.Sprintf("items=%d-%d", window.Offset, window.Offset+window.Limit-1),
	}
	for key, value := range headers {
		rangeHeaders[key] = value
	}

	var raw json.RawMessage
	header, err := c.getJSON(ctx, path, query, rangeHeaders, &raw)
	if err != nil {
		return Page[T]{}, err
	}

	items, err := decodeItems[T](raw, wrapKey)
	if err != nil {
		return Page[T]{}, fmt.Errorf(
			"decoding %s: %w",
			path,
			err,
		)
	}

	page := Page[T]{Items: items, Offset: window.Offset}
	if page.Items == nil {
		page.Items = []T{}
	}
	page.HasMore, page.Total = parseContentRange(header.Get("Content-Range"), len(items) == window.Limit)

	return page, nil
}

// decodeItems reads a JSON list, or the list stored under wrapKey when the
// endpoint wraps it with metadata. An empty body (204) yields no items.
func decodeItems[T any](
	raw json.RawMessage,
	wrapKey string,
) ([]T, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	if wrapKey != "" {
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wrapper); err != nil {
			return nil, err
		}
		raw = wrapper[wrapKey]
		if len(raw) == 0 {
			return nil, nil
		}
	}

	var items []T
	err := json.Unmarshal(raw, &items)

	return items, err
}

// parseContentRange reads headers such as "items 0-49/120" or "items 0-49/*",
// the latter meaning more items are available. When the header is missing,
// fullPage is used as a hint that another page may exist.
func parseContentRange(
	header string,
	fullPage bool,
) (bool, *int) {
	var start, end int
	var total string
	if _, err := fmt.Sscanf(header, "items %d-%d/%s", &start, &end, &total); err != nil {
		return fullPage, nil
	}

	if total == "*" {
		return true, nil
	}

	count, err := strconv.Atoi(total)
	if err != nil {
		return fullPage, nil
	}

	return end+1 < count, &count
}

func documentQuery(
	filter DocumentFilter,
) url.Values {
	query := url.Values{}
	if filter.Status != "" {
		query.Set("status", filter.Status)
	}
	if filter.EmissionDate != "" {
		query.Set("emission_date", filter.EmissionDate)
	}

	return query
}

func companyPath(
	companyID int64,
	suffix string,
) string {
	return "/companies/" + strconv.FormatInt(companyID, 10) + suffix
}

func (c *Client) getRaw(
	ctx context.Context,
	path string,
	headers map[string]string,
) (json.RawMessage, error) {
	var raw json.RawMessage
	_, err := c.getJSON(ctx, path, nil, headers, &raw)

	return raw, err
}

func (c *Client) getJSON(
	ctx context.Context,
	path string,
	query url.Values,
	headers map[string]string,
	out any,
) (http.Header, error) {
	return c.call(ctx, request{method: http.MethodGet, path: path, query: query, headers: headers}, out)
}

func jsonBody(
	payload any,
) ([]byte, error) {
	return json.Marshal(payload)
}

// sendJSON sends payload as a JSON body and decodes the response into out,
// which may be nil when the response is not needed.
func (c *Client) sendJSON(
	ctx context.Context,
	method string,
	path string,
	payload any,
	out any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = c.call(ctx, request{
		method:      method,
		path:        path,
		body:        body,
		contentType: "application/json",
	}, out)

	return err
}

type request struct {
	method      string
	path        string
	query       url.Values
	headers     map[string]string
	body        []byte
	contentType string
}

func (c *Client) call(
	ctx context.Context,
	req request,
	out any,
) (http.Header, error) {
	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return resp.Header, nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf(
			"decoding %s: %w",
			req.path,
			err,
		)
	}

	return resp.Header, nil
}

// do performs a request and retries once with a fresh token on 401.
func (c *Client) do(
	ctx context.Context,
	req request,
) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		resp, err := c.send(ctx, req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			_ = resp.Body.Close()
			c.tokens.Invalidate()
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("%s %s: %w", req.method, req.path, ErrNotFound)
		}

		if resp.StatusCode >= http.StatusBadRequest {
			defer func() { _ = resp.Body.Close() }()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
		}

		return resp, nil
	}
}

func (c *Client) send(
	ctx context.Context,
	req request,
) (*http.Response, error) {
	token, err := c.tokens.AccessToken(ctx)
	if err != nil {
		return nil, err
	}

	target := c.baseURL + req.path
	if len(req.query) > 0 {
		target += "?" + req.query.Encode()
	}

	var body io.Reader
	if req.body != nil {
		body = bytes.NewReader(req.body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.method, target, body)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/json")
	// Same application headers as the Tiime web application.
	httpReq.Header.Set("tiime-app", c.appHeader)
	httpReq.Header.Set("tiime-app-platform", "web")
	httpReq.Header.Set("tiime-app-version", c.appVersion)
	if req.contentType != "" {
		httpReq.Header.Set("Content-Type", req.contentType)
	}
	for key, value := range req.headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf(
			"calling Tiime API %s %s: %w",
			req.method,
			req.path,
			err,
		)
	}

	return resp, nil
}
