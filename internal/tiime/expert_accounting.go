package tiime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// movementsExpand is the expansion the expert application asks for movements.
const movementsExpand = "lettering,accounting_entry_counterpart,vat_amount,has_attachment"

// AccountingPeriods lists the accounting periods (fiscal years) of a client
// company.
func (c *Client) AccountingPeriods(
	ctx context.Context,
	companyID int64,
) (json.RawMessage, error) {
	return c.expertGet(ctx, expertCompanyPath(companyID, "/accounting_periods"), nil)
}

// Movements lists the accounting movements (entry lines) of a client company.
func (c *Client) Movements(
	ctx context.Context,
	companyID int64,
	query ExpertQuery,
	window Range,
) (RawPage, error) {
	values := query.values()
	values.Set("expand", movementsExpand)

	return c.expertList(ctx, expertCompanyPath(companyID, "/movements"), values, nil, window)
}

// AccountingEntryMovements lists the lines of an accounting entry.
func (c *Client) AccountingEntryMovements(
	ctx context.Context,
	companyID int64,
	entryID int64,
) (json.RawMessage, error) {
	return c.expertGet(ctx, expertCompanyPath(companyID, "/accounting_entries"+idPath(entryID)+"/movements"), nil)
}

// MovementTotals sums the movements of an account between two dates:
// count, total, total_debit and total_credit.
func (c *Client) MovementTotals(
	ctx context.Context,
	companyID int64,
	accountID int64,
	date string,
) (json.RawMessage, error) {
	query := url.Values{"pcg_entry": {strconv.FormatInt(accountID, 10)}, "date": {date}}

	return c.expertGet(ctx, expertCompanyPath(companyID, "/movements/cumuls"), query)
}

// SetMovementsValidated validates movements, or removes their validation.
func (c *Client) SetMovementsValidated(
	ctx context.Context,
	companyID int64,
	movementIDs []int64,
	validated bool,
) (json.RawMessage, error) {
	action := "/movements/unvalidate"
	if validated {
		action = "/movements/validate"
	}

	return c.expertSend(ctx, http.MethodPost, expertCompanyPath(companyID, action), nil, map[string]any{"movements": idRefs(movementIDs)})
}

// TrialBalance lists the account balances of an accounting period.
func (c *Client) TrialBalance(
	ctx context.Context,
	companyID int64,
	periodID int64,
	query ExpertQuery,
	window Range,
) (RawPage, error) {
	return c.expertList(ctx, expertCompanyPath(companyID, "/accounting_periods"+idPath(periodID)+"/balances"), query.values(), nil, window)
}

// AccountingResult returns the accounting result of a period.
func (c *Client) AccountingResult(
	ctx context.Context,
	companyID int64,
	periodID int64,
) (json.RawMessage, error) {
	query := url.Values{"items": {"accounting_result"}}

	return c.expertGet(ctx, expertCompanyPath(companyID, "/accounting_periods"+idPath(periodID)+"/results"), query)
}

// AccountBalances lists the balances of an account over the accounting
// periods.
func (c *Client) AccountBalances(
	ctx context.Context,
	companyID int64,
	accountID int64,
) (json.RawMessage, error) {
	return c.expertGet(ctx, expertCompanyPath(companyID, "/pcg_entries"+idPath(accountID)+"/balances"), nil)
}

// ChartOfAccounts lists the accounts of a client company.
func (c *Client) ChartOfAccounts(
	ctx context.Context,
	companyID int64,
	filters map[string]string,
) (json.RawMessage, error) {
	return c.expertGet(ctx, expertCompanyPath(companyID, "/pcg_entries"), ExpertQuery{Filters: filters}.values())
}

// Journals lists the accounting journals of a client company, optionally of
// one type.
func (c *Client) Journals(
	ctx context.Context,
	companyID int64,
	journalType string,
) (json.RawMessage, error) {
	return c.expertGet(ctx, expertCompanyPath(companyID, "/journals"), ExpertQuery{Filters: map[string]string{"type": journalType}}.values())
}

// CompanyLabels lists the categories (labels) of a client company.
func (c *Client) CompanyLabels(
	ctx context.Context,
	companyID int64,
	query ExpertQuery,
	window Range,
) (RawPage, error) {
	return c.expertList(ctx, expertCompanyPath(companyID, "/labels"), query.values(), nil, window)
}

// CompanyBankTransactions lists the bank transactions of a client company.
func (c *Client) CompanyBankTransactions(
	ctx context.Context,
	companyID int64,
	query ExpertQuery,
	window Range,
) (RawPage, error) {
	values := query.values()
	values.Set("expand", "imputations")
	headers := map[string]string{"Accept": "application/vnd.tiime.bank_transactions.v2+json"}

	return c.expertList(ctx, expertCompanyPath(companyID, "/bank_transactions"), values, headers, window)
}

// RequestReceipts asks the client for the receipts of bank transactions.
func (c *Client) RequestReceipts(
	ctx context.Context,
	companyID int64,
	transactionIDs []int64,
) (json.RawMessage, error) {
	payload := map[string]any{"type": "receipt", "bank_transactions": idRefs(transactionIDs)}

	return c.expertSend(ctx, http.MethodPost, expertCompanyPath(companyID, "/bank_transactions/accountant_detail_requests"), nil, payload)
}

// CancelReceiptRequests withdraws receipt requests.
func (c *Client) CancelReceiptRequests(
	ctx context.Context,
	companyID int64,
	requestIDs []int64,
) error {
	payload := map[string]any{"type": "receipt", "accountant_detail_requests": idRefs(requestIDs)}
	_, err := c.expertSend(ctx, http.MethodDelete, expertCompanyPath(companyID, "/bank_transactions/accountant_detail_requests"), nil, payload)

	return err
}
