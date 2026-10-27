package tools

import (
	"context"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// ListTransactionsInput filters bank transactions.
type ListTransactionsInput struct {
	CompanyInput
	PageInput
	Search        string `json:"search,omitempty" jsonschema:"Free text search on the wording, amount or comment"`
	BankAccountID int64  `json:"bank_account_id,omitempty" jsonschema:"Only transactions of this bank account, see list_bank_accounts"`
	Date          string `json:"date,omitempty" jsonschema:"Transaction date conditions separated by commas, e.g. >=2026-10-01,<=2026-10-31"`
	Matched       *bool  `json:"matched,omitempty" jsonschema:"false for transactions without any matched document or invoice (missing receipts), true for matched ones, omit for all"`
}

// AnnotateTransactionsInput describes the annotation of transactions.
type AnnotateTransactionsInput struct {
	CompanyInput
	TransactionIDs []int64  `json:"transaction_ids" jsonschema:"IDs of the bank transactions to annotate"`
	Comment        *string  `json:"comment,omitempty" jsonschema:"Comment replacing the current one, empty string to clear it"`
	LabelID        int64    `json:"label_id,omitempty" jsonschema:"Accounting label (category) to apply, see list_labels"`
	Tags           []string `json:"tags,omitempty" jsonschema:"Tag names replacing the current tags, empty list to clear them"`
}

// TransactionDocumentsInput links or unlinks documents.
type TransactionDocumentsInput struct {
	CompanyInput
	TransactionID int64   `json:"transaction_id" jsonschema:"ID of the bank transaction"`
	DocumentIDs   []int64 `json:"document_ids" jsonschema:"IDs of the documents, see list_documents"`
}

// TransactionInvoicesInput links or unlinks invoices.
type TransactionInvoicesInput struct {
	CompanyInput
	TransactionID int64   `json:"transaction_id" jsonschema:"ID of the bank transaction"`
	InvoiceIDs    []int64 `json:"invoice_ids" jsonschema:"IDs of the sales invoices, see list_invoices"`
}

// BankAccountsOutput lists bank accounts.
type BankAccountsOutput struct {
	BankAccounts []tiime.BankAccount `json:"bank_accounts"`
}

// LabelsOutput lists accounting labels.
type LabelsOutput struct {
	Labels []tiime.Label `json:"labels"`
}

// TagsOutput lists tags.
type TagsOutput struct {
	Tags []tiime.Tag `json:"tags"`
}

// MatchingsOutput lists what is matched with a transaction.
type MatchingsOutput struct {
	Matchings []tiime.Matching `json:"matchings"`
}

// AnnotateOutput confirms an annotation.
type AnnotateOutput struct {
	Updated int `json:"updated"`
}

func registerBanking(
	server *mcp.Server,
	h *Handler,
) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	idempotent := &mcp.ToolAnnotations{IdempotentHint: true}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_bank_accounts",
		Description: "List the bank accounts synchronized with Tiime, with their balance.",
		Annotations: readOnly,
	}, h.ListBankAccounts)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_bank_transactions",
		Description: "List bank transactions, most recent first, with their comment, tags, accounting labels and number of matched documents and invoices.",
		Annotations: readOnly,
	}, h.ListBankTransactions)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_bank_transaction",
		Description: "Get the full detail of a bank transaction, including its imputations and matched documents.",
		Annotations: readOnly,
	}, h.GetBankTransaction)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "annotate_bank_transactions",
		Description: "Set the comment, accounting label or tags of one or several bank transactions. Omitted fields are left unchanged.",
		Annotations: idempotent,
	}, h.AnnotateTransactions)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_transaction_documents",
		Description: "List the documents and invoices matched with a bank transaction.",
		Annotations: readOnly,
	}, h.ListTransactionDocuments)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "link_documents_to_transaction",
		Description: "Match documents (receipts...) with a bank transaction, keeping the documents already matched.",
		Annotations: idempotent,
	}, h.LinkDocuments)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "link_invoices_to_transaction",
		Description: "Match sales invoices with a bank transaction (e.g. a customer payment), keeping the invoices already matched.",
		Annotations: idempotent,
	}, h.LinkInvoices)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "unlink_invoices_from_transaction",
		Description: "Remove sales invoices from the ones matched with a bank transaction.",
		Annotations: idempotent,
	}, h.UnlinkInvoices)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "suggest_labels",
		Description: "Get the accounting labels (categories) Tiime suggests for a bank transaction, to annotate it with annotate_bank_transactions.",
		Annotations: readOnly,
	}, h.SuggestLabels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "unlink_documents_from_transaction",
		Description: "Remove documents from the ones matched with a bank transaction.",
		Annotations: idempotent,
	}, h.UnlinkDocuments)
}

// ListBankAccounts implements the list_bank_accounts tool.
func (h *Handler) ListBankAccounts(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CompanyInput,
) (*mcp.CallToolResult, BankAccountsOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, BankAccountsOutput{}, err
	}

	accounts, err := h.api.BankAccounts(ctx, companyID)

	return nil, BankAccountsOutput{BankAccounts: accounts}, err
}

// ListBankTransactions implements the list_bank_transactions tool.
func (h *Handler) ListBankTransactions(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListTransactionsInput,
) (*mcp.CallToolResult, tiime.Page[tiime.BankTransaction], error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.Page[tiime.BankTransaction]{}, err
	}

	filter := tiime.TransactionFilter{Search: in.Search, BankAccountID: in.BankAccountID, Date: in.Date, Matched: in.Matched}
	page, err := h.api.BankTransactions(ctx, companyID, filter, in.window())

	return nil, page, err
}

// GetBankTransaction implements the get_bank_transaction tool.
func (h *Handler) GetBankTransaction(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	transaction, err := h.api.BankTransaction(ctx, companyID, in.ID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(transaction), nil, nil
}

// AnnotateTransactions implements the annotate_bank_transactions tool.
func (h *Handler) AnnotateTransactions(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in AnnotateTransactionsInput,
) (*mcp.CallToolResult, AnnotateOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, AnnotateOutput{}, err
	}

	annotation := tiime.TransactionAnnotation{Comment: in.Comment, LabelID: in.LabelID, Tags: in.Tags}
	if err := h.api.AnnotateTransactions(ctx, companyID, in.TransactionIDs, annotation); err != nil {
		return nil, AnnotateOutput{}, err
	}

	return nil, AnnotateOutput{Updated: len(in.TransactionIDs)}, nil
}

// ListTransactionDocuments implements the list_transaction_documents tool.
func (h *Handler) ListTransactionDocuments(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, MatchingsOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, MatchingsOutput{}, err
	}

	matchings, err := h.api.TransactionMatchings(ctx, companyID, in.ID)

	return nil, MatchingsOutput{Matchings: matchings}, err
}

// LinkDocuments implements the link_documents_to_transaction tool.
func (h *Handler) LinkDocuments(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in TransactionDocumentsInput,
) (*mcp.CallToolResult, MatchingsOutput, error) {
	return h.updateDocuments(ctx, in, func(current []int64) []int64 {
		for _, id := range in.DocumentIDs {
			if !slices.Contains(current, id) {
				current = append(current, id)
			}
		}
		return current
	})
}

// UnlinkDocuments implements the unlink_documents_from_transaction tool.
func (h *Handler) UnlinkDocuments(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in TransactionDocumentsInput,
) (*mcp.CallToolResult, MatchingsOutput, error) {
	return h.updateDocuments(ctx, in, func(current []int64) []int64 {
		return slices.DeleteFunc(current, func(id int64) bool { return slices.Contains(in.DocumentIDs, id) })
	})
}

// LinkInvoices implements the link_invoices_to_transaction tool.
func (h *Handler) LinkInvoices(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in TransactionInvoicesInput,
) (*mcp.CallToolResult, MatchingsOutput, error) {
	return h.updateInvoices(ctx, in, func(current []int64) []int64 {
		for _, id := range in.InvoiceIDs {
			if !slices.Contains(current, id) {
				current = append(current, id)
			}
		}
		return current
	})
}

// UnlinkInvoices implements the unlink_invoices_from_transaction tool.
func (h *Handler) UnlinkInvoices(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in TransactionInvoicesInput,
) (*mcp.CallToolResult, MatchingsOutput, error) {
	return h.updateInvoices(ctx, in, func(current []int64) []int64 {
		return slices.DeleteFunc(current, func(id int64) bool { return slices.Contains(in.InvoiceIDs, id) })
	})
}

// SuggestLabels implements the suggest_labels tool.
func (h *Handler) SuggestLabels(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, LabelsOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, LabelsOutput{}, err
	}

	labels, err := h.api.LabelSuggestions(ctx, companyID, in.ID)

	return nil, LabelsOutput{Labels: labels}, err
}

// updateInvoices rewrites the invoices matched with a transaction, as Tiime
// replaces the whole set on every update.
func (h *Handler) updateInvoices(
	ctx context.Context,
	in TransactionInvoicesInput,
	change func(current []int64) []int64,
) (*mcp.CallToolResult, MatchingsOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, MatchingsOutput{}, err
	}

	matchings, err := h.api.TransactionMatchings(ctx, companyID, in.TransactionID)
	if err != nil {
		return nil, MatchingsOutput{}, err
	}

	var current []int64
	for _, matching := range matchings {
		if matching.Type == "invoice" {
			current = append(current, matching.ID)
		}
	}

	if err := h.api.SetTransactionInvoices(ctx, companyID, in.TransactionID, change(current)); err != nil {
		return nil, MatchingsOutput{}, err
	}

	matchings, err = h.api.TransactionMatchings(ctx, companyID, in.TransactionID)

	return nil, MatchingsOutput{Matchings: matchings}, err
}

// updateDocuments rewrites the documents matched with a transaction, as Tiime
// replaces the whole set on every update.
func (h *Handler) updateDocuments(
	ctx context.Context,
	in TransactionDocumentsInput,
	change func(current []int64) []int64,
) (*mcp.CallToolResult, MatchingsOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, MatchingsOutput{}, err
	}

	matchings, err := h.api.TransactionMatchings(ctx, companyID, in.TransactionID)
	if err != nil {
		return nil, MatchingsOutput{}, err
	}

	var current []int64
	for _, matching := range matchings {
		if matching.Type != "invoice" {
			current = append(current, matching.ID)
		}
	}

	if err := h.api.SetTransactionDocuments(ctx, companyID, in.TransactionID, change(current)); err != nil {
		return nil, MatchingsOutput{}, err
	}

	matchings, err = h.api.TransactionMatchings(ctx, companyID, in.TransactionID)

	return nil, MatchingsOutput{Matchings: matchings}, err
}
