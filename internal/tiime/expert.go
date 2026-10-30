package tiime

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
)

// The expert.tiime.fr application of accounting firms uses its own routes,
// under /expert. Its responses are returned as Tiime sends them: the tools
// expose them to the agent without a typed model, which keeps them working when
// Tiime adds fields.

// RawPage is a page of items kept as Tiime sends them.
type RawPage = Page[map[string]any]

// ExpertQuery holds the optional query parameters of an expert listing.
type ExpertQuery struct {
	// Search is the free text search, sent as q.
	Search string
	// Sort is "<field>:<asc|desc>", sent as sorts.
	Sort string
	// Filters are sent as is, for instance {"date": ">=2026-01-01"}.
	Filters map[string]string
}

func (q ExpertQuery) values() url.Values {
	values := url.Values{}
	if q.Search != "" {
		values.Set("q", q.Search)
	}
	if q.Sort != "" {
		values.Set("sorts", q.Sort)
	}
	for key, value := range q.Filters {
		if value != "" {
			values.Set(key, value)
		}
	}

	return values
}

func expertCompanyPath(
	companyID int64,
	suffix string,
) string {
	return "/expert/companies/" + strconv.FormatInt(companyID, 10) + suffix
}

func idPath(
	id int64,
) string {
	return "/" + strconv.FormatInt(id, 10)
}

// expertList reads a paginated expert listing.
func (c *Client) expertList(
	ctx context.Context,
	path string,
	query url.Values,
	headers map[string]string,
	window Range,
) (RawPage, error) {
	return getPage[map[string]any](ctx, c, path, query, headers, window, "")
}

// expertGet reads an expert resource.
func (c *Client) expertGet(
	ctx context.Context,
	path string,
	query url.Values,
) (json.RawMessage, error) {
	return c.expertGetWith(ctx, path, query, nil)
}

// expertGetWith reads an expert resource with extra headers, such as a
// versioned Accept header.
func (c *Client) expertGetWith(
	ctx context.Context,
	path string,
	query url.Values,
	headers map[string]string,
) (json.RawMessage, error) {
	var raw json.RawMessage
	_, err := c.getJSON(ctx, path, query, headers, &raw)

	return raw, err
}

// expertSend writes an expert resource and returns the response, empty when
// Tiime answers without a body.
func (c *Client) expertSend(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	payload any,
) (json.RawMessage, error) {
	req := request{method: method, path: path, query: query}
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		req.body = body
		req.contentType = "application/json"
	}

	var raw json.RawMessage
	if _, err := c.call(ctx, req, &raw); err != nil {
		return nil, err
	}

	return raw, nil
}

// expertDelete deletes an expert resource.
func (c *Client) expertDelete(
	ctx context.Context,
	path string,
	query url.Values,
) error {
	_, err := c.call(ctx, request{method: http.MethodDelete, path: path, query: query}, nil)

	return err
}

// expertDownload posts payload and returns the file Tiime answers with, and
// its name when Tiime gives one.
func (c *Client) expertDownload(
	ctx context.Context,
	path string,
	payload any,
) ([]byte, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	resp, err := c.do(ctx, request{
		method:      http.MethodPost,
		path:        path,
		body:        body,
		contentType: "application/json",
		headers:     map[string]string{"Accept": "*/*"},
	})
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	filename := ""
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
		filename = params["filename"]
	}

	return content, filename, nil
}
