// Package tools exposes Tiime as MCP tools: invoicing, banking, documents,
// expense reports, VAT, accounting labels and exchanges with the accounting firm.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// API is the subset of the Tiime client used by the tools.
type API interface {
	InvoicingAPI
	BankingAPI
	DocumentsAPI
	ExpensesAPI
	AccountantAPI
	VATAPI
	LabelsAPI
	InvoicingCycleAPI
	InsightsAPI
}

// InvoicingAPI reads companies, clients, invoices and quotes.
type InvoicingAPI interface {
	Companies(
		ctx context.Context,
	) ([]tiime.Company, error)
	Customers(
		ctx context.Context,
		companyID int64,
		filter tiime.CustomerFilter,
		window tiime.Range,
	) (tiime.Page[tiime.Customer], error)
	Customer(
		ctx context.Context,
		companyID int64,
		customerID int64,
	) (json.RawMessage, error)
	Contacts(
		ctx context.Context,
		companyID int64,
		customerID int64,
	) ([]tiime.Contact, error)
	Invoices(
		ctx context.Context,
		companyID int64,
		filter tiime.DocumentFilter,
		window tiime.Range,
	) (tiime.Page[tiime.Document], error)
	Invoice(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
	) (json.RawMessage, error)
	InvoicePDF(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
	) ([]byte, error)
	Quotes(
		ctx context.Context,
		companyID int64,
		filter tiime.DocumentFilter,
		window tiime.Range,
	) (tiime.Page[tiime.Document], error)
	Quote(
		ctx context.Context,
		companyID int64,
		quoteID int64,
	) (json.RawMessage, error)
	VATTypes(
		ctx context.Context,
	) ([]tiime.VATType, error)
	CreateInvoiceDraft(
		ctx context.Context,
		companyID int64,
		draft tiime.InvoiceDraft,
	) (json.RawMessage, error)
	UpdateInvoiceDraft(
		ctx context.Context,
		companyID int64,
		invoiceID int64,
		draft tiime.InvoiceDraft,
	) (json.RawMessage, error)
	InvoiceFromQuote(
		ctx context.Context,
		companyID int64,
		quoteID int64,
	) (json.RawMessage, error)
	CreateCustomer(
		ctx context.Context,
		companyID int64,
		input tiime.CustomerInput,
	) (json.RawMessage, error)
	UpdateCustomer(
		ctx context.Context,
		companyID int64,
		customerID int64,
		input tiime.CustomerInput,
	) (json.RawMessage, error)
}

// BankingAPI reads and annotates bank transactions.
type BankingAPI interface {
	BankAccounts(
		ctx context.Context,
		companyID int64,
	) ([]tiime.BankAccount, error)
	BankTransactions(
		ctx context.Context,
		companyID int64,
		filter tiime.TransactionFilter,
		window tiime.Range,
	) (tiime.Page[tiime.BankTransaction], error)
	BankTransaction(
		ctx context.Context,
		companyID int64,
		transactionID int64,
	) (json.RawMessage, error)
	TransactionMatchings(
		ctx context.Context,
		companyID int64,
		transactionID int64,
	) ([]tiime.Matching, error)
	SetTransactionDocuments(
		ctx context.Context,
		companyID int64,
		transactionID int64,
		documentIDs []int64,
	) error
	AnnotateTransactions(
		ctx context.Context,
		companyID int64,
		transactionIDs []int64,
		annotation tiime.TransactionAnnotation,
	) error
	SetTransactionInvoices(
		ctx context.Context,
		companyID int64,
		transactionID int64,
		invoiceIDs []int64,
	) error
	LabelSuggestions(
		ctx context.Context,
		companyID int64,
		transactionID int64,
	) ([]tiime.Label, error)
	AllBankTransactions(
		ctx context.Context,
		companyID int64,
		filter tiime.TransactionFilter,
		limit int,
	) ([]tiime.BankTransaction, bool, error)
}

// DocumentsAPI manages the documents space.
type DocumentsAPI interface {
	DocumentCategories(
		ctx context.Context,
		companyID int64,
	) ([]tiime.DocumentCategory, error)
	AccountingDocuments(
		ctx context.Context,
		companyID int64,
		filter tiime.AccountingDocumentFilter,
		window tiime.Range,
	) (tiime.Page[tiime.AccountingDocument], error)
	UploadDocument(
		ctx context.Context,
		companyID int64,
		categoryID int64,
		filename string,
		content []byte,
	) (tiime.AccountingDocument, error)
	UpdateDocument(
		ctx context.Context,
		companyID int64,
		documentID int64,
		update tiime.DocumentUpdate,
	) (tiime.AccountingDocument, error)
	DeleteDocument(
		ctx context.Context,
		companyID int64,
		documentID int64,
	) error
	DownloadDocument(
		ctx context.Context,
		companyID int64,
		documentID int64,
	) ([]byte, string, error)
}

// ExpensesAPI manages expense reports.
type ExpensesAPI interface {
	ExpenseReports(
		ctx context.Context,
		companyID int64,
		window tiime.Range,
	) (tiime.Page[tiime.ExpenseReport], error)
	ExpenseReport(
		ctx context.Context,
		companyID int64,
		reportID int64,
	) (json.RawMessage, error)
	CreateExpenseReport(
		ctx context.Context,
		companyID int64,
		input tiime.ExpenseReportInput,
	) (json.RawMessage, error)
	UpdateExpenseReport(
		ctx context.Context,
		companyID int64,
		reportID int64,
		update tiime.ExpenseReportUpdate,
	) (json.RawMessage, error)
	Vehicles(
		ctx context.Context,
		companyID int64,
	) (json.RawMessage, error)
	Travels(
		ctx context.Context,
		companyID int64,
		window tiime.Range,
	) (tiime.Page[map[string]any], error)
	CreateTravel(
		ctx context.Context,
		companyID int64,
		input tiime.TravelInput,
	) (json.RawMessage, error)
	EstimateTravelAmount(
		ctx context.Context,
		companyID int64,
		vehicleID int64,
		distance float64,
	) (json.RawMessage, error)
	CreateMileageReport(
		ctx context.Context,
		companyID int64,
		input tiime.ExpenseReportInput,
	) (json.RawMessage, error)
}

// Handler implements the MCP tools on top of the Tiime API.
type Handler struct {
	api              API
	auth             Auth
	defaultCompanyID int64
	downloadDir      string

	companyMu sync.Mutex
	companyID int64
}

// NewHandler creates a Handler. defaultCompanyID may be zero, in which case the
// company is detected when the account has access to a single one.
func NewHandler(
	api API,
	auth Auth,
	defaultCompanyID int64,
	downloadDir string,
) *Handler {
	return &Handler{
		api:              api,
		auth:             auth,
		defaultCompanyID: defaultCompanyID,
		downloadDir:      downloadDir,
	}
}

// CompanyInput selects the company a tool works on.
type CompanyInput struct {
	CompanyID int64 `json:"company_id,omitempty" jsonschema:"Tiime company ID, see list_companies. Optional when TIIME_COMPANY_ID is set or the account has a single company"`
}

// PageInput selects a window of results.
type PageInput struct {
	Offset int `json:"offset,omitempty" jsonschema:"Number of items to skip, defaults to 0"`
	Limit  int `json:"limit,omitempty" jsonschema:"Maximum number of items to return, defaults to 50, at most 100"`
}

// ListDocumentsInput filters invoice and quote listings.
type ListDocumentsInput struct {
	CompanyInput
	PageInput
	Status       string `json:"status,omitempty" jsonschema:"Filter on the status field, e.g. draft or paid"`
	EmissionDate string `json:"emission_date,omitempty" jsonschema:"Emission date conditions separated by commas, e.g. >=2026-01-01,<2026-02-01 or a single date 2026-01-15"`
}

// ListCustomersInput filters client listings.
type ListCustomersInput struct {
	CompanyInput
	PageInput
	Archived *bool `json:"archived,omitempty" jsonschema:"true for archived clients only, false for active clients only, omit for both"`
}

// GetByIDInput identifies a single resource.
type GetByIDInput struct {
	CompanyInput
	ID int64 `json:"id" jsonschema:"Tiime ID of the resource"`
}

// CompaniesOutput lists companies.
type CompaniesOutput struct {
	Companies []tiime.Company `json:"companies"`
}

// CustomerOutput is a client with its contacts.
type CustomerOutput struct {
	Customer map[string]any  `json:"client"`
	Contacts []tiime.Contact `json:"contacts"`
}

// PDFOutput reports where an invoice PDF was saved.
type PDFOutput struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

// Register adds the tools to the MCP server.
func Register(
	server *mcp.Server,
	h *Handler,
) {
	registerAuth(server, h.auth)
	registerInvoicing(server, h)
	registerBanking(server, h)
	registerDocuments(server, h)
	registerExpenses(server, h)
	registerCashflow(server, h)
	registerAccountant(server, h)
	registerInvoicingWrites(server, h)
	registerVAT(server, h)
	registerLabels(server, h)
	registerInvoicingCycle(server, h)
	registerInsights(server, h)
}

func registerInvoicing(
	server *mcp.Server,
	h *Handler,
) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_companies",
		Description: "List the Tiime companies the account has access to, with their IDs.",
		Annotations: readOnly,
	}, h.ListCompanies)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_clients",
		Description: "List the invoicing clients of a company, with billed amounts and outstanding balance.",
		Annotations: readOnly,
	}, h.ListCustomers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_client",
		Description: "Get the full detail of a client and its contacts.",
		Annotations: readOnly,
	}, h.GetCustomer)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_invoices",
		Description: "List the invoices of a company, filterable by status and emission date.",
		Annotations: readOnly,
	}, h.ListInvoices)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_invoice",
		Description: "Get the full detail of an invoice, including its lines, as returned by Tiime.",
		Annotations: readOnly,
	}, h.GetInvoice)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "download_invoice_pdf",
		Description: "Download the PDF of an invoice to the local download directory and return its path.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.DownloadInvoicePDF)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_quotes",
		Description: "List the quotes of a company, filterable by status and emission date.",
		Annotations: readOnly,
	}, h.ListQuotes)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_quote",
		Description: "Get the full detail of a quote, including its lines, as returned by Tiime.",
		Annotations: readOnly,
	}, h.GetQuote)
}

// ListCompanies implements the list_companies tool.
func (h *Handler) ListCompanies(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ struct{},
) (*mcp.CallToolResult, CompaniesOutput, error) {
	companies, err := h.api.Companies(ctx)
	if err != nil {
		return nil, CompaniesOutput{}, err
	}

	return nil, CompaniesOutput{Companies: companies}, nil
}

// ListCustomers implements the list_clients tool.
func (h *Handler) ListCustomers(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListCustomersInput,
) (*mcp.CallToolResult, tiime.Page[tiime.Customer], error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.Page[tiime.Customer]{}, err
	}

	filter := tiime.CustomerFilter{}
	if in.Archived != nil {
		filter.Archived = fmt.Sprint(*in.Archived)
	}

	page, err := h.api.Customers(ctx, companyID, filter, in.window())

	return nil, page, err
}

// GetCustomer implements the get_client tool.
func (h *Handler) GetCustomer(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, CustomerOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, CustomerOutput{}, err
	}

	customer, err := h.api.Customer(ctx, companyID, in.ID)
	if err != nil {
		return nil, CustomerOutput{}, err
	}

	contacts, err := h.api.Contacts(ctx, companyID, in.ID)
	if err != nil {
		return nil, CustomerOutput{}, err
	}

	var detail map[string]any
	if err := json.Unmarshal(customer, &detail); err != nil {
		return nil, CustomerOutput{}, fmt.Errorf(
			"decoding client: %w",
			err,
		)
	}

	return nil, CustomerOutput{Customer: detail, Contacts: contacts}, nil
}

// ListInvoices implements the list_invoices tool.
func (h *Handler) ListInvoices(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListDocumentsInput,
) (*mcp.CallToolResult, tiime.Page[tiime.Document], error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.Page[tiime.Document]{}, err
	}

	page, err := h.api.Invoices(ctx, companyID, in.filter(), in.window())

	return nil, page, err
}

// GetInvoice implements the get_invoice tool.
func (h *Handler) GetInvoice(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	invoice, err := h.api.Invoice(ctx, companyID, in.ID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(invoice), nil, nil
}

// DownloadInvoicePDF implements the download_invoice_pdf tool.
func (h *Handler) DownloadInvoicePDF(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, PDFOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, PDFOutput{}, err
	}

	content, err := h.api.InvoicePDF(ctx, companyID, in.ID)
	if err != nil {
		return nil, PDFOutput{}, err
	}

	path, err := h.save(fmt.Sprintf("tiime-invoice-%d.pdf", in.ID), content)
	if err != nil {
		return nil, PDFOutput{}, err
	}

	return nil, PDFOutput{Path: path, Bytes: len(content)}, nil
}

// save writes a downloaded file into the download directory.
func (h *Handler) save(
	name string,
	content []byte,
) (string, error) {
	return saveFile(h.downloadDir, name, content)
}

// saveFile writes a downloaded file into dir.
func saveFile(
	dir string,
	name string,
	content []byte,
) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf(
			"creating download directory: %w",
			err,
		)
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", fmt.Errorf(
			"writing %s: %w",
			name,
			err,
		)
	}

	return path, nil
}

// ListQuotes implements the list_quotes tool.
func (h *Handler) ListQuotes(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListDocumentsInput,
) (*mcp.CallToolResult, tiime.Page[tiime.Document], error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.Page[tiime.Document]{}, err
	}

	page, err := h.api.Quotes(ctx, companyID, in.filter(), in.window())

	return nil, page, err
}

// GetQuote implements the get_quote tool.
func (h *Handler) GetQuote(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	quote, err := h.api.Quote(ctx, companyID, in.ID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(quote), nil, nil
}

// resolveCompany picks the company from the tool input, the configuration, or
// the account itself when it has access to exactly one company.
func (h *Handler) resolveCompany(
	ctx context.Context,
	requested int64,
) (int64, error) {
	if requested != 0 {
		return requested, nil
	}
	if h.defaultCompanyID != 0 {
		return h.defaultCompanyID, nil
	}

	h.companyMu.Lock()
	defer h.companyMu.Unlock()

	if h.companyID != 0 {
		return h.companyID, nil
	}

	companies, err := h.api.Companies(ctx)
	if err != nil {
		return 0, err
	}

	if len(companies) != 1 {
		names := make([]string, 0, len(companies))
		for _, company := range companies {
			names = append(names, fmt.Sprintf("%s (%d)", company.Name, company.ID))
		}
		return 0, fmt.Errorf(
			"company_id is required, the account has %d companies: %s",
			len(companies),
			strings.Join(names, ", "),
		)
	}

	h.companyID = companies[0].ID

	return h.companyID, nil
}

const maxLimit = 100

func (in PageInput) window() tiime.Range {
	limit := in.Limit
	if limit > maxLimit {
		limit = maxLimit
	}

	return tiime.Range{Offset: max(in.Offset, 0), Limit: limit}
}

func (in ListDocumentsInput) filter() tiime.DocumentFilter {
	return tiime.DocumentFilter{Status: in.Status, EmissionDate: in.EmissionDate}
}

func rawResult(
	raw json.RawMessage,
) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}},
	}
}
