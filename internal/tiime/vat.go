package tiime

import (
	"context"
	"net/url"
	"strconv"
)

// VATEstimate is the VAT Tiime computes for a declaration period.
type VATEstimate struct {
	Status            string  `json:"status,omitempty"`
	DetailedStatus    string  `json:"detailed_status,omitempty"`
	PeriodStartDate   string  `json:"period_start_date,omitempty"`
	PeriodEndDate     string  `json:"period_end_date,omitempty"`
	VATCollected      float64 `json:"vat_collected"`
	VATDeductible     float64 `json:"vat_deductible"`
	VATReportable     float64 `json:"vat_reportable"`
	VATToRefund       float64 `json:"vat_to_refund"`
	VATCreditToReport float64 `json:"vat_credit_to_report"`
	Taxes             float64 `json:"taxes"`
	TotalDue          float64 `json:"total_due"`
	DueBefore         string  `json:"due_before,omitempty"`
	SentDate          string  `json:"sent_date,omitempty"`
	DeclarationID     int64   `json:"vat_declaration_id,omitempty"`
}

// VATEstimate returns the VAT estimate of a month.
func (c *Client) VATEstimate(
	ctx context.Context,
	companyID int64,
	year int,
	month int,
) (VATEstimate, error) {
	query := url.Values{"year": {strconv.Itoa(year)}, "month": {strconv.Itoa(month)}}

	var estimate VATEstimate
	_, err := c.getJSON(ctx, accountsPath(companyID, "/estimated_vat"), query, nil, &estimate)

	return estimate, err
}

// VATMovement is an entry behind a collected or deductible VAT amount.
type VATMovement struct {
	ID      int64   `json:"id,omitempty"`
	Date    string  `json:"date"`
	Wording string  `json:"wording"`
	Amount  float64 `json:"amount"`
	// Source is bank_transaction, receipt, expense_report, invoice,
	// external_invoice, cash_register_cash_flow or manual_entry.
	Source  string `json:"source"`
	PieceID int64  `json:"piece_id,omitempty"`
}

// VATMovements lists the entries behind the collected or deductible VAT of a
// month. kind is "collected" or "deductible".
func (c *Client) VATMovements(
	ctx context.Context,
	companyID int64,
	year int,
	month int,
	kind string,
	window Range,
) (Page[VATMovement], error) {
	query := url.Values{"year": {strconv.Itoa(year)}, "month": {strconv.Itoa(month)}}

	return getPage[VATMovement](ctx, c, accountsPath(companyID, "/estimated_vat/"+url.PathEscape(kind)), query, nil, window, "results")
}
