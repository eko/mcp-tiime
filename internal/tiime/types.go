package tiime

// Company is a company the Tiime user has access to.
type Company struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Siret      string `json:"siret,omitempty"`
	LegalForm  string `json:"legal_form,omitempty"`
	Street     string `json:"street,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	City       string `json:"city,omitempty"`
	Country    string `json:"country,omitempty"`
}

// Customer is an invoicing client of a company.
type Customer struct {
	ID                    int64   `json:"id"`
	Name                  string  `json:"name"`
	Email                 string  `json:"email,omitempty"`
	Phone                 string  `json:"phone,omitempty"`
	Address               string  `json:"address,omitempty"`
	PostalCode            string  `json:"postal_code,omitempty"`
	City                  string  `json:"city,omitempty"`
	CountryCode           string  `json:"country_code,omitempty"`
	Siret                 string  `json:"siret,omitempty"`
	Professional          bool    `json:"professional"`
	Archived              bool    `json:"archived"`
	PaymentStatus         string  `json:"payment_status,omitempty"`
	BilledExcludingTaxes  float64 `json:"billed_excluding_taxes"`
	BilledIncludingTaxes  float64 `json:"billed_including_taxes"`
	BalanceIncludingTaxes float64 `json:"balance_including_taxes"`
}

// Contact is a person attached to a customer.
type Contact struct {
	ID        int64  `json:"id"`
	Firstname string `json:"firstname,omitempty"`
	Lastname  string `json:"lastname,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Job       string `json:"job,omitempty"`
}

// Tag is a free label attached to transactions, documents, invoices or quotes.
type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Type is empty for free tags, or invoice_client, customer, accountant, person.
	Type        string `json:"type,omitempty"`
	IsAutomatic bool   `json:"is_automatic,omitempty"`
	ArchivedAt  string `json:"archived_at,omitempty"`
}

// Document is the summary of an invoice or a quote, as returned by list endpoints.
type Document struct {
	ID                  int64   `json:"id"`
	CompiledNumber      string  `json:"compiled_number,omitempty"`
	Title               string  `json:"title,omitempty"`
	Status              string  `json:"status"`
	EmissionDate        string  `json:"emission_date,omitempty"`
	ClientID            int64   `json:"client_id,omitempty"`
	ClientName          string  `json:"client_name,omitempty"`
	TotalExcludingTaxes float64 `json:"total_excluding_taxes"`
	TotalIncludingTaxes float64 `json:"total_including_taxes"`
	Tags                []Tag   `json:"tags,omitempty"`
}

// DocumentFilter narrows invoice and quote listings.
type DocumentFilter struct {
	// Status matches the document status, e.g. "draft" or "paid".
	Status string
	// EmissionDate is a comma separated list of conditions such as
	// ">=2026-01-01,<2026-02-01" or a single date "2026-01-15".
	EmissionDate string
}

// CustomerFilter narrows customer listings.
type CustomerFilter struct {
	// Archived is "true", "false" or empty for all customers.
	Archived string
}

// Range selects a window of items, Tiime paginating through the Range header.
type Range struct {
	Offset int
	Limit  int
}

// Page is a window of items returned by a paginated endpoint.
type Page[T any] struct {
	Items   []T  `json:"items"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
	// Total is the overall item count when Tiime reports it.
	Total *int `json:"total,omitempty"`
}

// BankAccount is a bank account synchronized with Tiime.
type BankAccount struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name,omitempty"`
	BankName        string  `json:"bank_name,omitempty"`
	IBAN            string  `json:"iban,omitempty"`
	BalanceAmount   float64 `json:"balance_amount"`
	BalanceCurrency string  `json:"balance_currency,omitempty"`
	BalanceDate     string  `json:"balance_date,omitempty"`
	Enabled         bool    `json:"enabled"`
	Closed          bool    `json:"closed"`
}

// Label is an accounting label (category) applied to transactions and documents.
type Label struct {
	ID           int64  `json:"id,omitempty"`
	Label        string `json:"label,omitempty"`
	Name         string `json:"name,omitempty"`
	Acronym      string `json:"acronym,omitempty"`
	Color        string `json:"color,omitempty"`
	Disabled     bool   `json:"disabled"`
	IsStandard   bool   `json:"is_standard"`
	RegistryType string `json:"registry_type,omitempty"`
	RulesCount   int    `json:"rules_count,omitempty"`
}

// LabelRule assigns a label to the transactions whose wording contains Wording.
type LabelRule struct {
	ID              int64    `json:"id"`
	CreatedAt       string   `json:"created_at,omitempty"`
	Label           *Label   `json:"label,omitempty"`
	Wording         string   `json:"wording"`
	ExcludedWord    string   `json:"excluded_word,omitempty"`
	MinAmount       *float64 `json:"min_amount,omitempty"`
	MaxAmount       *float64 `json:"max_amount,omitempty"`
	AmountDirection string   `json:"amount_direction,omitempty"`
}

// Imputation is the share of a transaction assigned to an accounting label.
type Imputation struct {
	ID             int64   `json:"id"`
	Amount         float64 `json:"amount"`
	Label          *Label  `json:"label,omitempty"`
	CountDocuments int     `json:"count_documents"`
	CountInvoices  int     `json:"count_invoices"`
}

// BankTransaction is the summary of a bank transaction.
type BankTransaction struct {
	ID              int64        `json:"id"`
	TransactionDate string       `json:"transaction_date,omitempty"`
	RealizationDate string       `json:"realization_date,omitempty"`
	Wording         string       `json:"wording,omitempty"`
	OriginalWording string       `json:"original_wording,omitempty"`
	Amount          float64      `json:"amount"`
	Currency        string       `json:"currency,omitempty"`
	OperationType   string       `json:"operation_type,omitempty"`
	Comment         string       `json:"comment,omitempty"`
	CountDocuments  int          `json:"count_documents"`
	CountInvoices   int          `json:"count_invoices"`
	Tags            []Tag        `json:"tags,omitempty"`
	Imputations     []Imputation `json:"imputations,omitempty"`
}

// TransactionFilter narrows bank transaction listings.
type TransactionFilter struct {
	// Search is a free text search on the wording, amount or comment.
	Search        string
	BankAccountID int64
	// Date is a comma separated list of conditions on the transaction date,
	// e.g. ">=2026-10-01,<=2026-10-31".
	Date string
	// Matched keeps transactions with (true) or without (false) a matched
	// document or invoice when set.
	Matched *bool
}

// TransactionAnnotation describes the changes applied to transactions. Nil or
// zero fields are left untouched.
type TransactionAnnotation struct {
	Comment *string
	LabelID int64
	// Tags replaces the tags of the transactions when not nil.
	Tags []string
}

// Matching is a document or an invoice matched with a bank transaction.
type Matching struct {
	ID int64 `json:"id"`
	// Type is "invoice" or the category identifier of the document, e.g. "receipt".
	Type   string  `json:"type"`
	Name   string  `json:"name,omitempty"`
	Amount float64 `json:"amount"`
	Date   string  `json:"date,omitempty"`
}

// DocumentCategory groups accounting documents, e.g. receipts or advanced expenses.
type DocumentCategory struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier,omitempty"`
}

// Metadata is a typed value attached to an accounting document, such as its
// date, wording, amount or VAT amount.
type Metadata struct {
	Key   string `json:"key"`
	Type  string `json:"type,omitempty"`
	Value any    `json:"value"`
}

// AccountingDocument is a file stored in the Tiime documents space.
type AccountingDocument struct {
	ID            int64             `json:"id"`
	Name          string            `json:"name,omitempty"`
	MimeType      string            `json:"mime_type,omitempty"`
	CreatedAt     string            `json:"created_at,omitempty"`
	Comment       string            `json:"comment,omitempty"`
	Category      *DocumentCategory `json:"category,omitempty"`
	Label         *Label            `json:"label,omitempty"`
	Tags          []Tag             `json:"tags,omitempty"`
	PaymentStatus string            `json:"payment_status,omitempty"`
	// CountLinkedEntities counts the transactions or expense reports the
	// document is attached to.
	CountLinkedEntities int `json:"count_linked_entities"`

	// Date, Wording, Amount, VATAmount and Currency are read from Metadata
	// by Summarize.
	Date      string   `json:"date,omitempty"`
	Wording   string   `json:"wording,omitempty"`
	Amount    *float64 `json:"amount,omitempty"`
	VATAmount *float64 `json:"vat_amount,omitempty"`
	Currency  string   `json:"currency,omitempty"`

	// IsPDP marks electronic invoices received through the Tiime platform.
	IsPDP             bool     `json:"is_pdp,omitempty"`
	PDPTypeLabel      string   `json:"pdp_type_label,omitempty"`
	PDPIsSelfBilling  bool     `json:"pdp_is_self_billing,omitempty"`
	PDPIsCreditNote   bool     `json:"pdp_is_credit_note,omitempty"`
	PDPRemainingToPay *float64 `json:"pdp_remaining_to_pay,omitempty"`
	PDPStatusMotif    string   `json:"pdp_status_motif,omitempty"`

	Metadata []Metadata `json:"metadata,omitempty"`
}

// Summarize fills the accounting fields from the raw metadata.
func (d *AccountingDocument) Summarize() {
	for _, entry := range d.Metadata {
		switch entry.Key {
		case "date":
			if date, ok := entry.Value.(string); ok && len(date) >= 10 {
				d.Date = date[:10]
			}
		case "wording":
			d.Wording, _ = entry.Value.(string)
		case "amount":
			d.Amount = amountPointer(entry.Value)
			if amount, ok := entry.Value.(map[string]any); ok {
				d.Currency, _ = amount["currency"].(string)
			}
		case "vat_amount":
			d.VATAmount = amountPointer(vatTotal(entry.Value))
		}
	}
}

// AccountingDocumentFilter narrows accounting document listings.
type AccountingDocumentFilter struct {
	CategoryID int64
	Search     string
}

// DocumentUpdate describes the accounting information set on a document. Nil
// or empty fields are left untouched.
type DocumentUpdate struct {
	// Date is formatted as YYYY-MM-DD.
	Date      string
	Wording   string
	Amount    *float64
	VATAmount *float64
	// Currency defaults to EUR.
	Currency string
	Comment  *string
}

// ExpenseReport is the summary of an expense report.
type ExpenseReport struct {
	ID              int64  `json:"id"`
	Name            string `json:"name,omitempty"`
	Date            string `json:"date,omitempty"`
	ExpenseType     string `json:"expense_type,omitempty"`
	LifecycleStatus string `json:"lifecycle_status,omitempty"`
	PaymentStatus   string `json:"payment_status,omitempty"`
	Comment         string `json:"comment,omitempty"`
	// Amount and VATAmount are read from the metadata, which is not returned.
	Amount    float64    `json:"amount"`
	VATAmount float64    `json:"vat_amount"`
	Metadata  []Metadata `json:"metadata,omitempty"`
}

// ExpenseReportInput describes an expense report made of advanced expenses.
type ExpenseReportInput struct {
	Name string
	// Date is formatted as YYYY-MM-DD.
	Date string
	// DocumentIDs are the advanced expense documents of an expense report.
	DocumentIDs []int64
	// TravelIDs are the trips of a mileage allowance report.
	TravelIDs []int64
	Comment   string
	// Submit saves the report instead of keeping it as a draft.
	Submit bool
}

// ExpenseReportUpdate describes changes on an expense report. Empty fields
// are left unchanged.
type ExpenseReportUpdate struct {
	Name    string
	Date    string
	Comment *string
	// AddDocumentIDs and RemoveDocumentIDs change the advanced expenses.
	AddDocumentIDs    []int64
	RemoveDocumentIDs []int64
	// Submit saves a draft report.
	Submit bool
}

// Address is a postal address of a mileage allowance trip.
type Address struct {
	Street     string `json:"street,omitempty" jsonschema:"Street"`
	PostalCode string `json:"postal_code,omitempty" jsonschema:"Postal code"`
	City       string `json:"city" jsonschema:"City"`
	Country    string `json:"country,omitempty" jsonschema:"Country"`
}

// TravelInput describes a trip for mileage allowances.
type TravelInput struct {
	Date               string
	Distance           float64
	VehicleID          int64
	Departure          Address
	Arrival            Address
	ArrivalCompanyName string
	RoundTrip          bool
	Comment            string
}
