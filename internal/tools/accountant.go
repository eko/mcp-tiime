package tools

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// AccountantAPI exchanges with the accounting firm.
type AccountantAPI interface {
	ToDoItems(
		ctx context.Context,
		companyID int64,
		types []string,
		window tiime.Range,
	) (tiime.Page[json.RawMessage], error)
	AnswerDocumentRequest(
		ctx context.Context,
		companyID int64,
		requestID int64,
		documentIDs []int64,
	) (json.RawMessage, error)
	DeclareDocumentUnavailable(
		ctx context.Context,
		companyID int64,
		transactionID int64,
		requestID int64,
		reason string,
		message string,
	) (json.RawMessage, error)
	Threads(
		ctx context.Context,
		companyID int64,
		status string,
		window tiime.Range,
	) (tiime.Page[json.RawMessage], error)
	Thread(
		ctx context.Context,
		companyID int64,
		threadID int64,
	) (json.RawMessage, error)
	PostThreadMessage(
		ctx context.Context,
		companyID int64,
		threadID int64,
		content string,
	) (json.RawMessage, error)
	MarkThreadRead(
		ctx context.Context,
		companyID int64,
		threadID int64,
	) error
}

// ToDoInput selects the to-do items.
type ToDoInput struct {
	CompanyInput
	PageInput
	Types []string `json:"types,omitempty" jsonschema:"Item types to keep: accountant_detail_request_receipt (missing receipts), accountant_detail_request_assessment, information_needed, thread_bank_transaction (unanswered messages), pending_advanced_expense, pending_travel. Omit for all"`
}

// AnswerDocumentRequestInput answers a document request.
type AnswerDocumentRequestInput struct {
	CompanyInput
	RequestID   int64   `json:"request_id" jsonschema:"ID of the document request (target_entity_id of a document_request item)"`
	DocumentIDs []int64 `json:"document_ids" jsonschema:"Documents answering the request, see upload_document and list_documents"`
}

// DeclareUnavailableInput declares a receipt unavailable.
type DeclareUnavailableInput struct {
	CompanyInput
	TransactionID int64  `json:"transaction_id" jsonschema:"ID of the bank transaction carrying the request"`
	RequestID     int64  `json:"request_id" jsonschema:"ID of the accountant detail request of the transaction"`
	Reason        string `json:"reason" jsonschema:"Why the document cannot be provided"`
	Message       string `json:"message,omitempty" jsonschema:"Message to the accounting firm"`
}

// ThreadsInput selects threads.
type ThreadsInput struct {
	CompanyInput
	PageInput
	Status string `json:"status,omitempty" jsonschema:"opened, closed, or omit for both"`
}

// PostMessageInput posts a message.
type PostMessageInput struct {
	CompanyInput
	ThreadID int64  `json:"thread_id" jsonschema:"ID of the thread, see list_threads"`
	Content  string `json:"content" jsonschema:"Message to send to the accounting firm"`
}

// MarkReadOutput confirms a thread was marked as read.
type MarkReadOutput struct {
	ThreadID int64 `json:"thread_id"`
}

func registerAccountant(
	server *mcp.Server,
	h *Handler,
) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_accountant_requests",
		Description: "List the to-do items from the accounting firm: missing receipts and information requests on transactions, " +
			"document requests, unanswered messages and pending expenses. A missing receipt is answered by matching a document " +
			"with link_documents_to_transaction, or with declare_document_unavailable.",
		Annotations: readOnly,
	}, h.ListAccountantRequests)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "answer_document_request",
		Description: "Answer a document request of the accounting firm with documents of the documents space.",
	}, h.AnswerDocumentRequest)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "declare_document_unavailable",
		Description: "Answer a receipt request on a bank transaction by declaring the document cannot be provided, with a reason.",
	}, h.DeclareDocumentUnavailable)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_threads",
		Description: "List the discussion threads with the accounting firm, most recent message first.",
		Annotations: readOnly,
	}, h.ListThreads)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_thread",
		Description: "Get a discussion thread with the accounting firm and its messages.",
		Annotations: readOnly,
	}, h.GetThread)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_thread_message",
		Description: "Send a message to the accounting firm in an existing thread.",
	}, h.PostThreadMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mark_thread_read",
		Description: "Mark a discussion thread with the accounting firm as read.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.MarkThreadRead)
}

// ListAccountantRequests implements the list_accountant_requests tool.
func (h *Handler) ListAccountantRequests(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ToDoInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	page, err := h.api.ToDoItems(ctx, companyID, in.Types, in.window())
	if err != nil {
		return nil, nil, err
	}

	return jsonResult(page)
}

// AnswerDocumentRequest implements the answer_document_request tool.
func (h *Handler) AnswerDocumentRequest(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in AnswerDocumentRequestInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	answer, err := h.api.AnswerDocumentRequest(ctx, companyID, in.RequestID, in.DocumentIDs)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(answer), nil, nil
}

// DeclareDocumentUnavailable implements the declare_document_unavailable tool.
func (h *Handler) DeclareDocumentUnavailable(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in DeclareUnavailableInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	request, err := h.api.DeclareDocumentUnavailable(ctx, companyID, in.TransactionID, in.RequestID, in.Reason, in.Message)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(request), nil, nil
}

// ListThreads implements the list_threads tool.
func (h *Handler) ListThreads(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ThreadsInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	page, err := h.api.Threads(ctx, companyID, in.Status, in.window())
	if err != nil {
		return nil, nil, err
	}

	return jsonResult(page)
}

// GetThread implements the get_thread tool.
func (h *Handler) GetThread(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	thread, err := h.api.Thread(ctx, companyID, in.ID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(thread), nil, nil
}

// PostThreadMessage implements the post_thread_message tool.
func (h *Handler) PostThreadMessage(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in PostMessageInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	message, err := h.api.PostThreadMessage(ctx, companyID, in.ThreadID, in.Content)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(message), nil, nil
}

// MarkThreadRead implements the mark_thread_read tool.
func (h *Handler) MarkThreadRead(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, MarkReadOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, MarkReadOutput{}, err
	}

	if err := h.api.MarkThreadRead(ctx, companyID, in.ID); err != nil {
		return nil, MarkReadOutput{}, err
	}

	return nil, MarkReadOutput{ThreadID: in.ID}, nil
}

// jsonResult returns a value whose shape is not modeled as JSON text.
func jsonResult(
	value any,
) (*mcp.CallToolResult, any, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(content), nil, nil
}
