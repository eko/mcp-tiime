package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// InvoicingCycleAPI sends, finalizes and settles invoices and quotes.
type InvoicingCycleAPI interface {
	CreateQuoteDraft(
		ctx context.Context,
		companyID int64,
		draft tiime.InvoiceDraft,
		validityPeriod string,
	) (json.RawMessage, error)
	UpdateQuote(
		ctx context.Context,
		companyID int64,
		quoteID int64,
		draft tiime.InvoiceDraft,
		validityPeriod string,
		status string,
	) (json.RawMessage, error)
	SendInvoice(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
		mail tiime.Mail,
	) error
	SendQuote(
		ctx context.Context,
		companyID int64,
		quoteID int64,
		mail tiime.Mail,
	) error
	SendReminder(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
		mail tiime.Mail,
	) error
	Reminders(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
	) (json.RawMessage, error)
	DuplicateInvoice(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
	) (json.RawMessage, error)
	DuplicateQuote(
		ctx context.Context,
		companyID int64,
		quoteID int64,
	) (json.RawMessage, error)
	DeleteInvoiceDraft(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
	) error
	DeleteQuote(
		ctx context.Context,
		companyID int64,
		quoteID int64,
	) error
	FinalizeInvoice(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
	) (json.RawMessage, error)
	CancelInvoice(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
		date string,
	) (json.RawMessage, error)
	CreateCreditNote(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
	) (json.RawMessage, error)
	RecordInvoicePayment(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
		payment tiime.InvoicePayment,
	) (json.RawMessage, error)
	InvoicePayments(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
	) (json.RawMessage, error)
	CancelInvoicePayment(
		ctx context.Context,
		companyID int64,
		paymentUUID string,
	) error
	Articles(
		ctx context.Context,
		companyID int64,
		search string,
		window tiime.Range,
	) (tiime.Page[json.RawMessage], error)
	SaveArticle(
		ctx context.Context,
		companyID int64,
		article tiime.Article,
	) (json.RawMessage, error)
	DeleteArticle(
		ctx context.Context,
		companyID int64,
		articleID int64,
	) error
	InvoicingUnits(
		ctx context.Context,
		companyID int64,
	) ([]tiime.InvoicingUnit, error)
}

// MailInput customizes an email sent by Tiime.
type MailInput struct {
	To       []string `json:"to,omitempty" jsonschema:"Recipients, defaults to the client billing email and contacts"`
	Cc       []string `json:"cc,omitempty" jsonschema:"Carbon copy recipients"`
	Subject  string   `json:"subject,omitempty" jsonschema:"Subject, defaults to the Tiime template"`
	Body     string   `json:"body,omitempty" jsonschema:"Body, defaults to the Tiime template"`
	CopyToMe bool     `json:"copy_to_me,omitempty" jsonschema:"true to receive a blind copy"`
}

// SendInput sends an invoice, a quote or a reminder.
type SendInput struct {
	CompanyInput
	MailInput
	ID int64 `json:"id" jsonschema:"ID of the invoice or quote"`
}

// QuoteDraftInput describes a quote.
type QuoteDraftInput struct {
	InvoiceDraftInput
	ValidityPeriod string `json:"validity_period,omitempty" jsonschema:"Validity, e.g. 30 days"`
}

// UpdateQuoteInput describes changes on a quote.
type UpdateQuoteInput struct {
	QuoteDraftInput
	ID     int64  `json:"id" jsonschema:"ID of the quote"`
	Status string `json:"status,omitempty" jsonschema:"accepted or refused to record the answer of the client"`
}

// CancelInvoiceInput cancels an invoice.
type CancelInvoiceInput struct {
	CompanyInput
	ID   int64  `json:"id" jsonschema:"ID of the invoice to cancel"`
	Date string `json:"date" jsonschema:"Emission date of the cancelling credit note, YYYY-MM-DD"`
}

// RecordPaymentInput records a payment.
type RecordPaymentInput struct {
	CompanyInput
	InvoiceID int64   `json:"invoice_id" jsonschema:"ID of the invoice"`
	Date      string  `json:"date" jsonschema:"Payment date, YYYY-MM-DD"`
	Amount    float64 `json:"amount" jsonschema:"Amount received, including taxes"`
	Method    string  `json:"method,omitempty" jsonschema:"bank_transfer (default), card, check, cash or levy"`
}

// CancelPaymentInput cancels a recorded payment.
type CancelPaymentInput struct {
	CompanyInput
	UUID string `json:"uuid" jsonschema:"UUID of the payment, see list_invoice_payments"`
}

// ListArticlesInput searches the catalogue.
type ListArticlesInput struct {
	CompanyInput
	PageInput
	Search string `json:"search,omitempty" jsonschema:"Text to search in the articles"`
}

// SaveArticleInput describes an article.
type SaveArticleInput struct {
	CompanyInput
	tiime.Article
}

// SentOutput confirms an email was sent.
type SentOutput struct {
	Sent int64 `json:"sent"`
}

func registerInvoicingCycle(
	server *mcp.Server,
	h *Handler,
) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := true

	mcp.AddTool(server, &mcp.Tool{
		Name:        "finalize_invoice",
		Description: "Number a draft invoice so it can be sent. A numbered invoice can no longer be changed, only cancelled with a credit note.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.FinalizeInvoice)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "send_invoice",
		Description: "Email an invoice to the client, by default to its billing email and contacts with the Tiime template.",
	}, h.SendInvoice)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "duplicate_invoice",
		Description: "Copy an invoice into a new draft, e.g. to bill the same service again.",
	}, h.DuplicateInvoice)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_invoice_draft",
		Description: "Delete an invoice that is still a draft.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.DeleteInvoiceDraft)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cancel_invoice",
		Description: "Cancel a numbered invoice by issuing a credit note for its full amount.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.CancelInvoice)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_credit_note",
		Description: "Create a credit note draft from an invoice, to refund all or part of it.",
	}, h.CreateCreditNote)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "record_invoice_payment",
		Description: "Record a payment received for an invoice outside a bank transaction (check, cash...). Bank payments are matched with link_invoices_to_transaction.",
	}, h.RecordInvoicePayment)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_invoice_payments",
		Description: "List the payments collected for an invoice.",
		Annotations: readOnly,
	}, h.ListInvoicePayments)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cancel_invoice_payment",
		Description: "Cancel a payment recorded manually on an invoice.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.CancelInvoicePayment)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "send_invoice_reminder",
		Description: "Email a reminder for an unpaid invoice, by default with the Tiime reminder template.",
	}, h.SendInvoiceReminder)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_invoice_reminders",
		Description: "List the reminders sent for an invoice.",
		Annotations: readOnly,
	}, h.ListInvoiceReminders)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_quote",
		Description: "Create a quote for a client. Layout and seller details are copied from the latest quote or invoice.",
	}, h.CreateQuote)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_quote",
		Description: "Update a quote that is not answered yet, or record the answer of the client (accepted or refused).",
	}, h.UpdateQuote)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "send_quote",
		Description: "Email a quote to the client, by default to its billing email and contacts with the Tiime template.",
	}, h.SendQuote)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "duplicate_quote",
		Description: "Copy a quote into a new one.",
	}, h.DuplicateQuote)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_quote",
		Description: "Delete a quote.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.DeleteQuote)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_articles",
		Description: "List the articles of the invoicing catalogue.",
		Annotations: readOnly,
	}, h.ListArticles)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "save_article",
		Description: "Create an article of the invoicing catalogue, or update it when id is set.",
	}, h.SaveArticle)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_article",
		Description: "Delete an article of the invoicing catalogue.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.DeleteArticle)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_invoicing_units",
		Description: "List the units usable on invoice lines, e.g. day or hour.",
		Annotations: readOnly,
	}, h.ListInvoicingUnits)
}

// FinalizeInvoice implements the finalize_invoice tool.
func (h *Handler) FinalizeInvoice(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.FinalizeInvoice(ctx, companyID, in.ID)
	})
}

// SendInvoice implements the send_invoice tool.
func (h *Handler) SendInvoice(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in SendInput,
) (*mcp.CallToolResult, SentOutput, error) {
	return h.send(ctx, in, h.api.SendInvoice)
}

// SendQuote implements the send_quote tool.
func (h *Handler) SendQuote(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in SendInput,
) (*mcp.CallToolResult, SentOutput, error) {
	return h.send(ctx, in, h.api.SendQuote)
}

// SendInvoiceReminder implements the send_invoice_reminder tool.
func (h *Handler) SendInvoiceReminder(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in SendInput,
) (*mcp.CallToolResult, SentOutput, error) {
	return h.send(ctx, in, h.api.SendReminder)
}

func (h *Handler) send(
	ctx context.Context,
	in SendInput,
	send func(ctx context.Context, companyID int64, id int64, mail tiime.Mail) error,
) (*mcp.CallToolResult, SentOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, SentOutput{}, err
	}

	mail := tiime.Mail{To: in.To, Cc: in.Cc, Subject: in.Subject, Body: in.Body, CopyToMe: in.CopyToMe}
	if err := send(ctx, companyID, in.ID, mail); err != nil {
		return nil, SentOutput{}, err
	}

	return nil, SentOutput{Sent: in.ID}, nil
}

// ListInvoiceReminders implements the list_invoice_reminders tool.
func (h *Handler) ListInvoiceReminders(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.Reminders(ctx, companyID, in.ID)
	})
}

// DuplicateInvoice implements the duplicate_invoice tool.
func (h *Handler) DuplicateInvoice(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.DuplicateInvoice(ctx, companyID, in.ID)
	})
}

// DuplicateQuote implements the duplicate_quote tool.
func (h *Handler) DuplicateQuote(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.DuplicateQuote(ctx, companyID, in.ID)
	})
}

// DeleteInvoiceDraft implements the delete_invoice_draft tool.
func (h *Handler) DeleteInvoiceDraft(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, DeleteOutput, error) {
	return h.deleteCall(ctx, in, h.api.DeleteInvoiceDraft)
}

// DeleteQuote implements the delete_quote tool.
func (h *Handler) DeleteQuote(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, DeleteOutput, error) {
	return h.deleteCall(ctx, in, h.api.DeleteQuote)
}

// DeleteArticle implements the delete_article tool.
func (h *Handler) DeleteArticle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, DeleteOutput, error) {
	return h.deleteCall(ctx, in, h.api.DeleteArticle)
}

// CancelInvoice implements the cancel_invoice tool.
func (h *Handler) CancelInvoice(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CancelInvoiceInput,
) (*mcp.CallToolResult, any, error) {
	if in.Date == "" {
		return nil, nil, errRequired("date")
	}

	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.CancelInvoice(ctx, companyID, in.ID, in.Date)
	})
}

// CreateCreditNote implements the create_credit_note tool.
func (h *Handler) CreateCreditNote(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.CreateCreditNote(ctx, companyID, in.ID)
	})
}

// RecordInvoicePayment implements the record_invoice_payment tool.
func (h *Handler) RecordInvoicePayment(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in RecordPaymentInput,
) (*mcp.CallToolResult, any, error) {
	if in.Date == "" || in.Amount <= 0 {
		return nil, nil, errRequired("date and a positive amount")
	}

	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.RecordInvoicePayment(ctx, companyID, in.InvoiceID, tiime.InvoicePayment{Date: in.Date, Amount: in.Amount, Method: in.Method})
	})
}

// ListInvoicePayments implements the list_invoice_payments tool.
func (h *Handler) ListInvoicePayments(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.InvoicePayments(ctx, companyID, in.ID)
	})
}

// CancelInvoicePayment implements the cancel_invoice_payment tool.
func (h *Handler) CancelInvoicePayment(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CancelPaymentInput,
) (*mcp.CallToolResult, CancelledOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, CancelledOutput{}, err
	}

	if err := h.api.CancelInvoicePayment(ctx, companyID, in.UUID); err != nil {
		return nil, CancelledOutput{}, err
	}

	return nil, CancelledOutput{Cancelled: in.UUID}, nil
}

// CreateQuote implements the create_quote tool.
func (h *Handler) CreateQuote(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in QuoteDraftInput,
) (*mcp.CallToolResult, any, error) {
	if in.ClientID == 0 || in.EmissionDate == "" || len(in.Lines) == 0 {
		return nil, nil, errRequired("client_id, emission_date and lines")
	}

	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.CreateQuoteDraft(ctx, companyID, in.draft(), in.ValidityPeriod)
	})
}

// UpdateQuote implements the update_quote tool.
func (h *Handler) UpdateQuote(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in UpdateQuoteInput,
) (*mcp.CallToolResult, any, error) {
	if in.Status != "" && in.Status != "accepted" && in.Status != "refused" {
		return nil, nil, fmt.Errorf("status must be accepted or refused, got %q", in.Status)
	}

	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.UpdateQuote(ctx, companyID, in.ID, in.draft(), in.ValidityPeriod, in.Status)
	})
}

// ListArticles implements the list_articles tool.
func (h *Handler) ListArticles(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListArticlesInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	page, err := h.api.Articles(ctx, companyID, in.Search, in.window())
	if err != nil {
		return nil, nil, err
	}

	return jsonResult(page)
}

// SaveArticle implements the save_article tool.
func (h *Handler) SaveArticle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in SaveArticleInput,
) (*mcp.CallToolResult, any, error) {
	if in.Code == "" {
		return nil, nil, errRequired("code")
	}

	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.SaveArticle(ctx, companyID, in.Article)
	})
}

// ListInvoicingUnits implements the list_invoicing_units tool.
func (h *Handler) ListInvoicingUnits(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CompanyInput,
) (*mcp.CallToolResult, InvoicingUnitsOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, InvoicingUnitsOutput{}, err
	}

	units, err := h.api.InvoicingUnits(ctx, companyID)

	return nil, InvoicingUnitsOutput{Units: units}, err
}

// InvoicingUnitsOutput lists invoicing units.
type InvoicingUnitsOutput struct {
	Units []tiime.InvoicingUnit `json:"units"`
}

// CancelledOutput confirms a cancellation.
type CancelledOutput struct {
	Cancelled string `json:"cancelled"`
}

// rawCall resolves the company and returns the raw Tiime response.
func (h *Handler) rawCall(
	ctx context.Context,
	requestedCompany int64,
	call func(companyID int64) (json.RawMessage, error),
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, requestedCompany)
	if err != nil {
		return nil, nil, err
	}

	raw, err := call(companyID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(raw), nil, nil
}

// deleteCall resolves the company and deletes a resource.
func (h *Handler) deleteCall(
	ctx context.Context,
	in GetByIDInput,
	remove func(ctx context.Context, companyID int64, id int64) error,
) (*mcp.CallToolResult, DeleteOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, DeleteOutput{}, err
	}

	if err := remove(ctx, companyID, in.ID); err != nil {
		return nil, DeleteOutput{}, err
	}

	return nil, DeleteOutput{Deleted: in.ID}, nil
}
