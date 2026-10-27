package tiime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ToDoItems lists the pending requests of the accounting firm and other
// items to handle: unanswered threads, missing receipts, pending expenses.
// types restricts the item types, e.g. "accountant_detail_request_receipt".
func (c *Client) ToDoItems(
	ctx context.Context,
	companyID int64,
	types []string,
	window Range,
) (Page[json.RawMessage], error) {
	query := url.Values{}
	if len(types) > 0 {
		query.Set("to_do_list_items_type", strings.Join(types, "|"))
	}
	headers := map[string]string{"Accept": "application/vnd.tiime.todo.module_type_with_mileage_expense+json"}

	return getPage[json.RawMessage](ctx, c, accountsPath(companyID, "/to_do_list_items"), query, headers, window, "")
}

// AnswerDocumentRequest answers a document request of the accounting firm
// with documents of the documents space.
func (c *Client) AnswerDocumentRequest(
	ctx context.Context,
	companyID int64,
	requestID int64,
	documentIDs []int64,
) (json.RawMessage, error) {
	var raw json.RawMessage
	payload := map[string]any{"document_ids": documentIDs}
	path := accountsPath(companyID, "/accountant_detail_request/document_requests/"+strconv.FormatInt(requestID, 10))
	err := c.sendJSON(ctx, http.MethodPatch, path, payload, &raw)

	return raw, err
}

// DeclareDocumentUnavailable answers a receipt request on a transaction by
// declaring that the document cannot be provided.
func (c *Client) DeclareDocumentUnavailable(
	ctx context.Context,
	companyID int64,
	transactionID int64,
	requestID int64,
	reason string,
	message string,
) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{
		"document_declared_unavailable_at":     time.Now().UTC().Format(time.RFC3339),
		"document_declared_unavailable_reason": reason,
		"message_content":                      message,
	})
	if err != nil {
		return nil, err
	}

	var raw json.RawMessage
	path := transactionPath(companyID, transactionID, "/accountant_detail_requests/"+strconv.FormatInt(requestID, 10)+"/document_declared_unavailable")
	_, err = c.call(ctx, request{
		method:      http.MethodPatch,
		path:        path,
		headers:     map[string]string{"Accept": "application/vnd.tiime.accountant_request.document_unavailable.v2+json"},
		body:        body,
		contentType: "application/json",
	}, &raw)

	return raw, err
}

// Threads lists the discussion threads with the accounting firm, most recent
// message first. status is "opened", "closed" or empty for both.
func (c *Client) Threads(
	ctx context.Context,
	companyID int64,
	status string,
	window Range,
) (Page[json.RawMessage], error) {
	query := url.Values{
		"types": {"bank_transaction|ai_assistant"},
		"sorts": {"message_created_at:desc,created_at:desc"},
	}
	if status == "" {
		query.Set("status", "opened|closed")
	} else {
		query.Set("status", status)
	}

	return getPage[json.RawMessage](ctx, c, accountsPath(companyID, "/threads"), query, nil, window, "")
}

// Thread returns a thread with its messages.
func (c *Client) Thread(
	ctx context.Context,
	companyID int64,
	threadID int64,
) (json.RawMessage, error) {
	return c.getRaw(ctx, threadPath(companyID, threadID, ""), nil)
}

// PostThreadMessage posts a message in an existing thread.
func (c *Client) PostThreadMessage(
	ctx context.Context,
	companyID int64,
	threadID int64,
	content string,
) (json.RawMessage, error) {
	var raw json.RawMessage
	err := c.sendJSON(ctx, http.MethodPost, threadPath(companyID, threadID, "/messages"), map[string]string{"content": content}, &raw)

	return raw, err
}

// MarkThreadRead marks a thread as read.
func (c *Client) MarkThreadRead(
	ctx context.Context,
	companyID int64,
	threadID int64,
) error {
	return c.sendJSON(ctx, http.MethodPost, threadPath(companyID, threadID, "/read"), true, nil)
}

func threadPath(
	companyID int64,
	threadID int64,
	suffix string,
) string {
	return accountsPath(companyID, "/threads/"+strconv.FormatInt(threadID, 10)+suffix)
}
