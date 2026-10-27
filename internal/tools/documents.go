package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// maxUploadSize mirrors a reasonable limit for receipts and invoices.
const maxUploadSize = 20 << 20

// ListAccountingDocumentsInput filters documents.
type ListAccountingDocumentsInput struct {
	CompanyInput
	PageInput
	CategoryID int64  `json:"category_id,omitempty" jsonschema:"Only documents of this category, see list_document_categories"`
	Search     string `json:"search,omitempty" jsonschema:"Free text search"`
}

// DocumentInfoInput is the accounting information of a document.
type DocumentInfoInput struct {
	Date      string   `json:"date,omitempty" jsonschema:"Document date, YYYY-MM-DD"`
	Wording   string   `json:"wording,omitempty" jsonschema:"Wording, e.g. the supplier name"`
	Amount    *float64 `json:"amount,omitempty" jsonschema:"Amount including taxes"`
	VATAmount *float64 `json:"vat_amount,omitempty" jsonschema:"VAT amount"`
	Currency  string   `json:"currency,omitempty" jsonschema:"Currency code, defaults to EUR"`
	Comment   *string  `json:"comment,omitempty" jsonschema:"Comment on the document"`
}

// UploadDocumentInput describes a document to upload.
type UploadDocumentInput struct {
	CompanyInput
	DocumentInfoInput
	FilePath   string `json:"file_path" jsonschema:"Absolute path of the local file to upload (PDF or image)"`
	CategoryID int64  `json:"category_id,omitempty" jsonschema:"Target category ID, see list_document_categories. Takes precedence over category"`
	Category   string `json:"category,omitempty" jsonschema:"Target category identifier: receipt (default) for receipts, advanced_expense for expenses to be reimbursed in an expense report"`
}

// UpdateDocumentInput describes the changes on a document.
type UpdateDocumentInput struct {
	CompanyInput
	DocumentInfoInput
	ID int64 `json:"id" jsonschema:"Document ID"`
}

// DocumentCategoriesOutput lists categories.
type DocumentCategoriesOutput struct {
	Categories []tiime.DocumentCategory `json:"categories"`
}

// EInvoicesInput selects electronic invoices.
type EInvoicesInput struct {
	CompanyInput
	Limit int `json:"limit,omitempty" jsonschema:"Maximum number of e-invoices to return, defaults to 20"`
}

// EInvoicesOutput lists electronic invoices.
type EInvoicesOutput struct {
	EInvoices []tiime.AccountingDocument `json:"einvoices"`
	// Scanned is the number of recent documents searched.
	Scanned int `json:"scanned"`
}

// DownloadOutput reports where a document was saved.
type DownloadOutput struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

// DeleteOutput confirms a deletion.
type DeleteOutput struct {
	Deleted int64 `json:"deleted"`
}

func registerDocuments(
	server *mcp.Server,
	h *Handler,
) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := true

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_document_categories",
		Description: "List the categories of the documents space (receipts, advanced expenses, expense reports...).",
		Annotations: readOnly,
	}, h.ListDocumentCategories)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_documents",
		Description: "List the documents of the documents space, most recently added first, with their date, wording, amount, VAT and number of linked transactions or expense reports.",
		Annotations: readOnly,
	}, h.ListDocuments)

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_einvoices",
		Description: "List the electronic invoices received through the Tiime e-invoicing platform (PDP), most recent first, " +
			"with their type (invoice, self-billed invoice, credit note), payment status and remaining amount to pay.",
		Annotations: readOnly,
	}, h.ListEInvoices)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "download_document",
		Description: "Download the file of a document from the documents space to the local download directory and return its path.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.DownloadDocument)

	mcp.AddTool(server, &mcp.Tool{
		Name: "upload_document",
		Description: "Upload a local file (receipt, invoice...) to the Tiime documents space and optionally set its date, wording, amount and VAT. " +
			"Use category advanced_expense for expenses to include in an expense report.",
	}, h.UploadDocument)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_document",
		Description: "Set the date, wording, amount, VAT or comment of a document. Omitted fields are left unchanged.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.UpdateDocument)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_document",
		Description: "Delete a document from the Tiime documents space.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.DeleteDocument)
}

// ListDocumentCategories implements the list_document_categories tool.
func (h *Handler) ListDocumentCategories(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CompanyInput,
) (*mcp.CallToolResult, DocumentCategoriesOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, DocumentCategoriesOutput{}, err
	}

	categories, err := h.api.DocumentCategories(ctx, companyID)

	return nil, DocumentCategoriesOutput{Categories: categories}, err
}

// ListDocuments implements the list_documents tool.
func (h *Handler) ListDocuments(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListAccountingDocumentsInput,
) (*mcp.CallToolResult, tiime.Page[tiime.AccountingDocument], error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.Page[tiime.AccountingDocument]{}, err
	}

	filter := tiime.AccountingDocumentFilter{CategoryID: in.CategoryID, Search: in.Search}
	page, err := h.api.AccountingDocuments(ctx, companyID, filter, in.window())

	return nil, page, err
}

// UploadDocument implements the upload_document tool.
func (h *Handler) UploadDocument(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in UploadDocumentInput,
) (*mcp.CallToolResult, tiime.AccountingDocument, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.AccountingDocument{}, err
	}

	content, err := readUpload(in.FilePath)
	if err != nil {
		return nil, tiime.AccountingDocument{}, err
	}

	categoryID, err := h.resolveCategory(ctx, companyID, in.CategoryID, in.Category)
	if err != nil {
		return nil, tiime.AccountingDocument{}, err
	}

	document, err := h.api.UploadDocument(ctx, companyID, categoryID, filepath.Base(in.FilePath), content)
	if err != nil {
		return nil, tiime.AccountingDocument{}, err
	}

	update := in.update()
	if update == (tiime.DocumentUpdate{Currency: update.Currency}) {
		return nil, summarized(document), nil
	}

	updated, err := h.api.UpdateDocument(ctx, companyID, document.ID, update)
	if err != nil {
		return nil, tiime.AccountingDocument{}, fmt.Errorf(
			"document %d uploaded but its information could not be set: %w",
			document.ID,
			err,
		)
	}

	return nil, summarized(updated), nil
}

// UpdateDocument implements the update_document tool.
func (h *Handler) UpdateDocument(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in UpdateDocumentInput,
) (*mcp.CallToolResult, tiime.AccountingDocument, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.AccountingDocument{}, err
	}

	document, err := h.api.UpdateDocument(ctx, companyID, in.ID, in.update())

	return nil, summarized(document), err
}

// summarized replaces the raw metadata of a document by its accounting fields.
func summarized(
	document tiime.AccountingDocument,
) tiime.AccountingDocument {
	document.Summarize()
	document.Metadata = nil

	return document
}

// DeleteDocument implements the delete_document tool.
func (h *Handler) DeleteDocument(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, DeleteOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, DeleteOutput{}, err
	}

	if err := h.api.DeleteDocument(ctx, companyID, in.ID); err != nil {
		return nil, DeleteOutput{}, err
	}

	return nil, DeleteOutput{Deleted: in.ID}, nil
}

// maxEInvoiceScan bounds the documents searched for e-invoices, as Tiime
// offers no server side filter for them.
const maxEInvoiceScan = 500

// ListEInvoices implements the list_einvoices tool.
func (h *Handler) ListEInvoices(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in EInvoicesInput,
) (*mcp.CallToolResult, EInvoicesOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, EInvoicesOutput{}, err
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}

	output := EInvoicesOutput{EInvoices: []tiime.AccountingDocument{}}
	for output.Scanned < maxEInvoiceScan && len(output.EInvoices) < limit {
		page, err := h.api.AccountingDocuments(ctx, companyID, tiime.AccountingDocumentFilter{}, tiime.Range{Offset: output.Scanned, Limit: maxLimit})
		if err != nil {
			return nil, EInvoicesOutput{}, err
		}
		output.Scanned += len(page.Items)

		for _, document := range page.Items {
			if document.IsPDP && len(output.EInvoices) < limit {
				output.EInvoices = append(output.EInvoices, document)
			}
		}
		if !page.HasMore || len(page.Items) == 0 {
			break
		}
	}

	return nil, output, nil
}

// DownloadDocument implements the download_document tool.
func (h *Handler) DownloadDocument(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, DownloadOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, DownloadOutput{}, err
	}

	content, filename, err := h.api.DownloadDocument(ctx, companyID, in.ID)
	if err != nil {
		return nil, DownloadOutput{}, err
	}

	path, err := h.save(fmt.Sprintf("tiime-document-%d-%s", in.ID, safeName(filename)), content)
	if err != nil {
		return nil, DownloadOutput{}, err
	}

	return nil, DownloadOutput{Path: path, Bytes: len(content)}, nil
}

// resolveCategory returns categoryID when set, or the ID of the category with
// the given identifier, "receipt" by default.
func (h *Handler) resolveCategory(
	ctx context.Context,
	companyID int64,
	categoryID int64,
	identifier string,
) (int64, error) {
	if categoryID != 0 {
		return categoryID, nil
	}
	if identifier == "" {
		identifier = "receipt"
	}

	categories, err := h.api.DocumentCategories(ctx, companyID)
	if err != nil {
		return 0, err
	}

	for _, category := range categories {
		if category.Identifier == identifier {
			return category.ID, nil
		}
	}

	return 0, fmt.Errorf("no document category with identifier %q, see list_document_categories", identifier)
}

func (in DocumentInfoInput) update() tiime.DocumentUpdate {
	return tiime.DocumentUpdate{
		Date:      in.Date,
		Wording:   in.Wording,
		Amount:    in.Amount,
		VATAmount: in.VATAmount,
		Currency:  in.Currency,
		Comment:   in.Comment,
	}
}

// safeName keeps the base name of a file name given by Tiime, so it cannot
// escape the download directory.
func safeName(
	filename string,
) string {
	name := filepath.Base(filepath.Clean("/" + filename))
	if name == "/" || name == "." {
		return "file"
	}

	return name
}

func readUpload(
	path string,
) ([]byte, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("file_path must be absolute, got %q", path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > maxUploadSize {
		return nil, fmt.Errorf("%s is larger than %d MB", path, maxUploadSize>>20)
	}

	return os.ReadFile(path)
}
