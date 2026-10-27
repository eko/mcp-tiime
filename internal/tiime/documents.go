package tiime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
)

// documentsAccept matches the representation requested by the web application.
const documentsAccept = "application/vnd.tiime.doc.pdp+json"

// DocumentCategories lists the categories of the documents space.
func (c *Client) DocumentCategories(
	ctx context.Context,
	companyID int64,
) ([]DocumentCategory, error) {
	var categories []DocumentCategory
	_, err := c.getJSON(ctx, accountsPath(companyID, "/document_categories"), nil, nil, &categories)

	return categories, err
}

// AccountingDocuments lists the documents of the documents space.
func (c *Client) AccountingDocuments(
	ctx context.Context,
	companyID int64,
	filter AccountingDocumentFilter,
	window Range,
) (Page[AccountingDocument], error) {
	query := url.Values{}
	if filter.CategoryID != 0 {
		query.Set("document_categories", strconv.FormatInt(filter.CategoryID, 10))
	}
	if filter.Search != "" {
		query.Set("q", filter.Search)
	}
	// Without explicit sort, Tiime does not return the latest documents first.
	query.Set("sorts", "created_at:desc")
	headers := map[string]string{"Accept": documentsAccept}

	page, err := getPage[AccountingDocument](ctx, c, accountsPath(companyID, "/documents"), query, headers, window, "")
	for i := range page.Items {
		page.Items[i].Summarize()
		page.Items[i].Metadata = nil
	}

	return page, err
}

// DownloadDocument downloads the file of a document and returns its content
// with the file name given by Tiime.
func (c *Client) DownloadDocument(
	ctx context.Context,
	companyID int64,
	documentID int64,
) ([]byte, string, error) {
	resp, err := c.do(ctx, request{method: http.MethodGet, path: documentPath(companyID, documentID) + "/file", headers: map[string]string{"Accept": "*/*"}})
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

// AccountingDocument returns a document with its metadata.
func (c *Client) AccountingDocument(
	ctx context.Context,
	companyID int64,
	documentID int64,
) (AccountingDocument, error) {
	var document AccountingDocument
	_, err := c.getJSON(ctx, documentPath(companyID, documentID), nil, map[string]string{"Accept": documentsAccept}, &document)
	document.Summarize()

	return document, err
}

// UploadDocument uploads a file into a document category.
func (c *Client) UploadDocument(
	ctx context.Context,
	companyID int64,
	categoryID int64,
	filename string,
	content []byte,
) (AccountingDocument, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", filename)
	if err != nil {
		return AccountingDocument{}, err
	}
	if _, err := part.Write(content); err != nil {
		return AccountingDocument{}, err
	}
	if err := form.WriteField("source", "web"); err != nil {
		return AccountingDocument{}, err
	}
	if err := form.Close(); err != nil {
		return AccountingDocument{}, err
	}

	var document AccountingDocument
	path := accountsPath(companyID, "/document_categories/"+strconv.FormatInt(categoryID, 10)+"/documents")
	_, err = c.call(ctx, request{
		method:      http.MethodPost,
		path:        path,
		headers:     map[string]string{"Accept": documentsAccept},
		body:        body.Bytes(),
		contentType: form.FormDataContentType(),
	}, &document)

	return document, err
}

// UpdateDocument sets the accounting information of a document. Existing
// metadata are kept and only the provided values are replaced.
func (c *Client) UpdateDocument(
	ctx context.Context,
	companyID int64,
	documentID int64,
	update DocumentUpdate,
) (AccountingDocument, error) {
	var current struct {
		Metadata []map[string]any `json:"metadata"`
	}
	headers := map[string]string{"Accept": documentsAccept}
	if _, err := c.getJSON(ctx, documentPath(companyID, documentID), nil, headers, &current); err != nil {
		return AccountingDocument{}, err
	}

	currency := update.Currency
	if currency == "" {
		currency = "EUR"
	}

	metadata := current.Metadata
	if update.Date != "" {
		// Tiime stores dates as datetime values, e.g. "2026-10-08 00:00:00".
		metadata = setMetadata(metadata, "date", "datetime", update.Date+" 00:00:00")
	}
	if update.Wording != "" {
		metadata = setMetadata(metadata, "wording", "string", update.Wording)
	}
	if update.Amount != nil {
		metadata = setMetadata(metadata, "amount", "amount", map[string]any{"value": *update.Amount, "currency": currency})
	}
	if update.VATAmount != nil {
		metadata = setMetadata(metadata, "vat_amount", "amount", map[string]any{"value": *update.VATAmount, "currency": currency})
	}

	payload := map[string]any{"metadata": metadata}
	if update.Comment != nil {
		payload["comment"] = *update.Comment
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return AccountingDocument{}, err
	}

	var document AccountingDocument
	_, err = c.call(ctx, request{
		method:      http.MethodPatch,
		path:        documentPath(companyID, documentID),
		headers:     headers,
		body:        body,
		contentType: "application/json",
	}, &document)

	return document, err
}

// DeleteDocument deletes a document.
func (c *Client) DeleteDocument(
	ctx context.Context,
	companyID int64,
	documentID int64,
) error {
	_, err := c.call(ctx, request{method: http.MethodDelete, path: documentPath(companyID, documentID)}, nil)

	return err
}

// setMetadata replaces the value of key, keeping the type and extra fields
// Tiime already stores for it, or appends a new entry with defaultType.
func setMetadata(
	metadata []map[string]any,
	key string,
	defaultType string,
	value any,
) []map[string]any {
	for _, entry := range metadata {
		if entry["key"] != key {
			continue
		}
		// Amounts split by VAT rate are stored as a list holding a "total" entry.
		if entry["type"] == "amounts" {
			value = []map[string]any{{"key": "total", "value": value}}
		}
		entry["value"] = value
		return metadata
	}

	return append(metadata, map[string]any{"key": key, "type": defaultType, "value": value})
}

func documentPath(
	companyID int64,
	documentID int64,
) string {
	return accountsPath(companyID, fmt.Sprintf("/documents/%d", documentID))
}
