package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// ExpertAccountingAPI reads the accounting of a client company and asks the
// client for receipts.
type ExpertAccountingAPI interface {
	AccountingPeriods(
		ctx context.Context,
		companyID int64,
	) (json.RawMessage, error)
	Movements(
		ctx context.Context,
		companyID int64,
		query tiime.ExpertQuery,
		window tiime.Range,
	) (tiime.RawPage, error)
	AccountingEntryMovements(
		ctx context.Context,
		companyID int64,
		entryID int64,
	) (json.RawMessage, error)
	MovementTotals(
		ctx context.Context,
		companyID int64,
		accountID int64,
		date string,
	) (json.RawMessage, error)
	SetMovementsValidated(
		ctx context.Context,
		companyID int64,
		movementIDs []int64,
		validated bool,
	) (json.RawMessage, error)
	TrialBalance(
		ctx context.Context,
		companyID int64,
		periodID int64,
		query tiime.ExpertQuery,
		window tiime.Range,
	) (tiime.RawPage, error)
	AccountingResult(
		ctx context.Context,
		companyID int64,
		periodID int64,
	) (json.RawMessage, error)
	AccountBalances(
		ctx context.Context,
		companyID int64,
		accountID int64,
	) (json.RawMessage, error)
	ChartOfAccounts(
		ctx context.Context,
		companyID int64,
		filters map[string]string,
	) (json.RawMessage, error)
	Journals(
		ctx context.Context,
		companyID int64,
		journalType string,
	) (json.RawMessage, error)
	CompanyLabels(
		ctx context.Context,
		companyID int64,
		query tiime.ExpertQuery,
		window tiime.Range,
	) (tiime.RawPage, error)
	CompanyBankTransactions(
		ctx context.Context,
		companyID int64,
		query tiime.ExpertQuery,
		window tiime.Range,
	) (tiime.RawPage, error)
	RequestReceipts(
		ctx context.Context,
		companyID int64,
		transactionIDs []int64,
	) (json.RawMessage, error)
	CancelReceiptRequests(
		ctx context.Context,
		companyID int64,
		requestIDs []int64,
	) error
}

var (
	movementSortFields     = []string{"date", "journal_code", "piece_reference", "wording", "debit", "credit", "lettering"}
	trialBalanceSortFields = []string{"account_number", "solde", "previous_solde", "amount_evolution", "percent_evolution"}
	journalTypes           = []string{"unknown", "bank", "sales", "purchases", "cash_register", "various_operations", "stock", "a_nouveau", "overturn", "situation", "expense_report", "pay"}
	balanceSides           = []string{"debit", "credit"}
)

// PeriodInput selects dates directly or through an accounting period.
type PeriodInput struct {
	AccountingPeriodID int64  `json:"accounting_period_id,omitempty" jsonschema:"Accounting period (fiscal year), see list_accounting_periods. Sets from and to"`
	From               string `json:"from,omitempty" jsonschema:"On or after this date, YYYY-MM-DD"`
	To                 string `json:"to,omitempty" jsonschema:"On or before this date, YYYY-MM-DD"`
}

// ListMovementsInput filters the accounting movements of a client company.
type ListMovementsInput struct {
	ExpertCompanyInput
	PageInput
	PeriodInput
	AccountID      int64    `json:"account_id,omitempty" jsonschema:"Only this account (general ledger of the account), see list_chart_of_accounts"`
	AccountNumber  string   `json:"account_number,omitempty" jsonschema:"Exact account number, e.g. 401ACME"`
	AccountRoots   []string `json:"account_prefixes,omitempty" jsonschema:"Accounts starting with these roots, e.g. 401 or 6"`
	JournalCodes   []string `json:"journal_codes,omitempty" jsonschema:"Journal codes, e.g. AC or BQ1, see list_journals"`
	Amount         string   `json:"amount,omitempty" jsonschema:"Signed amount condition, credit positive and debit negative: 120.5, -120.5, >=100, <0 or >=100,<=200"`
	Wording        string   `json:"wording,omitempty" jsonschema:"Text in the movement label"`
	PieceReference string   `json:"piece_reference,omitempty" jsonschema:"Piece number"`
	Search         string   `json:"search,omitempty" jsonschema:"Free text search"`
	Lettered       *bool    `json:"lettered,omitempty" jsonschema:"Filter on lettering"`
	Validated      *bool    `json:"validated,omitempty" jsonschema:"Filter on validation (visa)"`
	HasAttachment  *bool    `json:"has_attachment,omitempty" jsonschema:"Filter on the presence of a receipt"`
	Sort           string   `json:"sort,omitempty" jsonschema:"<field>:<asc|desc>, field one of date, journal_code, piece_reference, wording, debit, credit, lettering"`
}

// EntryInput identifies an accounting entry.
type EntryInput struct {
	ExpertCompanyInput
	EntryID int64 `json:"entry_id" jsonschema:"Accounting entry ID, from accounting_entry.id of a movement"`
}

// AccountSummaryInput selects an account and optionally a period.
type AccountSummaryInput struct {
	ExpertCompanyInput
	PeriodInput
	AccountID int64 `json:"account_id" jsonschema:"Account ID, from pcg_entry.id of a movement or list_chart_of_accounts"`
}

// TrialBalanceInput selects the trial balance of an accounting period.
type TrialBalanceInput struct {
	ExpertCompanyInput
	PageInput
	AccountingPeriodID int64    `json:"accounting_period_id" jsonschema:"Accounting period (fiscal year), see list_accounting_periods"`
	AccountRoots       []string `json:"account_prefixes,omitempty" jsonschema:"Accounts starting with these roots, e.g. 4 or 512"`
	AccountNumbers     []string `json:"account_numbers,omitempty" jsonschema:"Exact account numbers"`
	BalanceSide        string   `json:"balance_side,omitempty" jsonschema:"debit for debit balances only, credit for credit balances only"`
	HasMovement        *bool    `json:"has_movement,omitempty" jsonschema:"Filter on accounts with movements"`
	Validated          *bool    `json:"validated,omitempty" jsonschema:"Filter on validated accounts"`
	Balanced           *bool    `json:"balanced,omitempty" jsonschema:"Filter on balanced (lettered) accounts"`
	AsOf               string   `json:"as_of,omitempty" jsonschema:"Balance at this date instead of the period end, YYYY-MM-DD"`
	Search             string   `json:"search,omitempty" jsonschema:"Free text search"`
	WithResult         bool     `json:"with_result,omitempty" jsonschema:"Also return the accounting result of the period"`
	Sort               string   `json:"sort,omitempty" jsonschema:"<field>:<asc|desc>, field one of account_number, solde, previous_solde, amount_evolution, percent_evolution"`
}

// TrialBalanceOutput is a page of account balances. The balance of a row is
// credit minus debit.
type TrialBalanceOutput struct {
	tiime.RawPage
	AccountingResult map[string]any `json:"accounting_result,omitempty"`
}

// ChartOfAccountsInput filters the accounts of a client company.
type ChartOfAccountsInput struct {
	ExpertCompanyInput
	AccountNumber      string `json:"account_number,omitempty" jsonschema:"Account number or start of it"`
	AccountingPeriodID int64  `json:"accounting_period_id,omitempty" jsonschema:"Accounting period, for the movement filters"`
	HasMovement        *bool  `json:"has_movement,omitempty" jsonschema:"Filter on accounts with movements"`
}

// JournalsInput filters the journals of a client company.
type JournalsInput struct {
	ExpertCompanyInput
	Type string `json:"type,omitempty" jsonschema:"bank, sales, purchases, cash_register, various_operations, stock, a_nouveau, overturn, situation, expense_report, pay or unknown"`
}

// CompanyLabelsInput filters the categories of a client company.
type CompanyLabelsInput struct {
	ExpertCompanyInput
	PageInput
	Search       string   `json:"search,omitempty" jsonschema:"Free text search"`
	AccountRoots []string `json:"account_prefixes,omitempty" jsonschema:"Categories whose account starts with these roots"`
	Disabled     *bool    `json:"disabled,omitempty" jsonschema:"Filter on disabled categories"`
}

// ValidateMovementsInput validates movements.
type ValidateMovementsInput struct {
	ExpertCompanyInput
	MovementIDs []int64 `json:"movement_ids" jsonschema:"Movement IDs, see list_movements"`
	Validated   bool    `json:"validated" jsonschema:"true to validate (visa), false to remove the validation"`
}

// ClientTransactionsInput filters the bank transactions of a client company.
type ClientTransactionsInput struct {
	ExpertCompanyInput
	PageInput
	Search        string `json:"search,omitempty" jsonschema:"Free text search"`
	BankAccountID int64  `json:"bank_account_id,omitempty" jsonschema:"Only this bank account"`
	LabelID       int64  `json:"label_id,omitempty" jsonschema:"Only this category, see list_company_labels"`
	From          string `json:"from,omitempty" jsonschema:"Transactions on or after this date, YYYY-MM-DD"`
}

// ReceiptRequestInput asks the client for receipts.
type ReceiptRequestInput struct {
	ExpertCompanyInput
	TransactionIDs []int64 `json:"transaction_ids" jsonschema:"Bank transactions needing a receipt, see list_client_bank_transactions"`
}

// CancelReceiptRequestInput withdraws receipt requests.
type CancelReceiptRequestInput struct {
	ExpertCompanyInput
	RequestIDs []int64 `json:"request_ids" jsonschema:"Receipt request IDs, from accountant_detail_requests[].id of a transaction"`
}

func registerExpertAccounting(
	server *mcp.Server,
	h *ExpertHandler,
) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_accounting_periods",
		Description: "List the accounting periods (fiscal years) of a client company, with their dates and whether they are closed.",
		Annotations: readOnly,
	}, h.ListAccountingPeriods)

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_movements",
		Description: "List the accounting movements (entry lines) of a client company, by period, account (general ledger), journal, amount, " +
			"lettering or validation. Amounts are signed: credit positive, debit negative.",
		Annotations: readOnly,
	}, h.ListMovements)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_accounting_entry",
		Description: "Get every line of an accounting entry.",
		Annotations: readOnly,
	}, h.GetAccountingEntry)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_account_summary",
		Description: "Get the balances of an account for each accounting period, and its debit and credit totals over a period.",
		Annotations: readOnly,
	}, h.GetAccountSummary)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_trial_balance",
		Description: "Get the trial balance (balance générale) of an accounting period, with filters on accounts and balance side, and optionally the accounting result.",
		Annotations: readOnly,
	}, h.GetTrialBalance)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_chart_of_accounts",
		Description: "List the accounts (chart of accounts) of a client company, with their IDs.",
		Annotations: readOnly,
	}, h.ListChartOfAccounts)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_journals",
		Description: "List the accounting journals of a client company, with their codes.",
		Annotations: readOnly,
	}, h.ListJournals)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_company_labels",
		Description: "List the categories (labels) of a client company, with their accounts and VAT settings.",
		Annotations: readOnly,
	}, h.ListCompanyLabels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "validate_movements",
		Description: "Validate (visa) accounting movements of a client company, or remove their validation.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.ValidateMovements)

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_client_bank_transactions",
		Description: "List the bank transactions of a client company, most recent first, with their categories, documents and " +
			"pending receipt requests (accountant_detail_requests).",
		Annotations: readOnly,
	}, h.ListClientBankTransactions)

	mcp.AddTool(server, &mcp.Tool{
		Name: "request_receipts",
		Description: "Ask the client for the receipts of bank transactions. The transactions appear in the client's to-do list in the " +
			"Tiime app; no email is sent.",
	}, h.RequestReceipts)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cancel_receipt_requests",
		Description: "Withdraw receipt requests sent to the client.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.CancelReceiptRequests)
}

// ListAccountingPeriods implements the list_accounting_periods tool.
func (h *ExpertHandler) ListAccountingPeriods(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ExpertCompanyInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	periods, err := h.api.AccountingPeriods(ctx, companyID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(periods), nil, nil
}

// ListMovements implements the list_movements tool.
func (h *ExpertHandler) ListMovements(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListMovementsInput,
) (*mcp.CallToolResult, tiime.RawPage, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, tiime.RawPage{}, err
	}
	period, err := h.period(ctx, companyID, in.PeriodInput)
	if err != nil {
		return nil, tiime.RawPage{}, err
	}
	sort, err := sortParam(in.Sort, movementSortFields)
	if err != nil {
		return nil, tiime.RawPage{}, err
	}
	if err := checkAmountCondition(in.Amount); err != nil {
		return nil, tiime.RawPage{}, err
	}

	filters := map[string]string{
		"date":                    period,
		"account_number":          in.AccountNumber,
		"account_number_prefixes": strings.Join(in.AccountRoots, "|"),
		"journal_code":            strings.Join(in.JournalCodes, ","),
		"amount":                  in.Amount,
		"wording":                 in.Wording,
		"piece_reference":         in.PieceReference,
		"lettered":                boolParam(in.Lettered),
		"validated":               boolParam(in.Validated),
		"has_attachment":          boolParam(in.HasAttachment),
	}
	if in.AccountID != 0 {
		filters["pcg_entry"] = strconv.FormatInt(in.AccountID, 10)
	}

	page, err := h.api.Movements(ctx, companyID, tiime.ExpertQuery{Search: in.Search, Sort: sort, Filters: filters}, in.window())

	return nil, page, err
}

// checkAmountCondition accepts conditions such as 12.5, -12.5, >=10 or
// >=10,<=20.
func checkAmountCondition(
	condition string,
) error {
	if condition == "" {
		return nil
	}

	for _, part := range strings.Split(condition, ",") {
		number := strings.TrimLeft(part, "<>=")
		if _, err := strconv.ParseFloat(number, 64); err != nil {
			return fmt.Errorf("invalid amount %q, expected e.g. 120.5, -120.5, >=100 or >=100,<=200", condition)
		}
	}

	return nil
}

// period builds the date filter from an accounting period or explicit dates.
func (h *ExpertHandler) period(
	ctx context.Context,
	companyID int64,
	in PeriodInput,
) (string, error) {
	if in.AccountingPeriodID == 0 {
		return dateRange(in.From, in.To)
	}
	if in.From != "" || in.To != "" {
		return "", errors.New("give accounting_period_id or from/to, not both")
	}

	raw, err := h.api.AccountingPeriods(ctx, companyID)
	if err != nil {
		return "", err
	}

	var periods []struct {
		ID        int64  `json:"id"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := json.Unmarshal(raw, &periods); err != nil {
		return "", fmt.Errorf(
			"decoding accounting periods: %w",
			err,
		)
	}
	for _, period := range periods {
		if period.ID == in.AccountingPeriodID {
			return dateRange(period.StartDate, period.EndDate)
		}
	}

	return "", fmt.Errorf("accounting period %d not found: see list_accounting_periods", in.AccountingPeriodID)
}

// GetAccountingEntry implements the get_accounting_entry tool.
func (h *ExpertHandler) GetAccountingEntry(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in EntryInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if in.EntryID == 0 {
		return nil, nil, errors.New("entry_id is required: see list_movements")
	}

	movements, err := h.api.AccountingEntryMovements(ctx, companyID, in.EntryID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(movements), nil, nil
}

// GetAccountSummary implements the get_account_summary tool.
func (h *ExpertHandler) GetAccountSummary(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in AccountSummaryInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if in.AccountID == 0 {
		return nil, nil, errors.New("account_id is required: see list_chart_of_accounts")
	}
	period, err := h.period(ctx, companyID, in.PeriodInput)
	if err != nil {
		return nil, nil, err
	}

	summary := map[string]json.RawMessage{}
	if summary["balances"], err = h.api.AccountBalances(ctx, companyID, in.AccountID); err != nil {
		return nil, nil, err
	}
	if period != "" {
		if summary["totals"], err = h.api.MovementTotals(ctx, companyID, in.AccountID, period); err != nil {
			return nil, nil, err
		}
	}

	return jsonResult(summary)
}

// GetTrialBalance implements the get_trial_balance tool.
func (h *ExpertHandler) GetTrialBalance(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in TrialBalanceInput,
) (*mcp.CallToolResult, TrialBalanceOutput, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, TrialBalanceOutput{}, err
	}
	if in.AccountingPeriodID == 0 {
		return nil, TrialBalanceOutput{}, errors.New("accounting_period_id is required: see list_accounting_periods")
	}
	if err := checkEnum("balance_side", in.BalanceSide, balanceSides); err != nil {
		return nil, TrialBalanceOutput{}, err
	}
	if err := checkDate(in.AsOf); err != nil {
		return nil, TrialBalanceOutput{}, err
	}
	sort, err := sortParam(in.Sort, trialBalanceSortFields)
	if err != nil {
		return nil, TrialBalanceOutput{}, err
	}

	filters := map[string]string{
		"account_number_prefixes": strings.Join(in.AccountRoots, "|"),
		"account_numbers":         strings.Join(in.AccountNumbers, "|"),
		"pcg_entry_has_movement":  boolParam(in.HasMovement),
		"pcg_entry_validated":     boolParam(in.Validated),
		"pcg_entry_balanced":      boolParam(in.Balanced),
	}
	// A balance is credit minus debit: debit balances are negative.
	switch in.BalanceSide {
	case "debit":
		filters["amount"] = "<0"
	case "credit":
		filters["amount"] = ">=0"
	}
	if in.AsOf != "" {
		filters["date"] = "<=" + in.AsOf
	}

	page, err := h.api.TrialBalance(ctx, companyID, in.AccountingPeriodID, tiime.ExpertQuery{Search: in.Search, Sort: sort, Filters: filters}, in.window())
	if err != nil {
		return nil, TrialBalanceOutput{}, err
	}
	out := TrialBalanceOutput{RawPage: page}

	if in.WithResult {
		result, err := h.api.AccountingResult(ctx, companyID, in.AccountingPeriodID)
		if err != nil {
			return nil, TrialBalanceOutput{}, err
		}
		if err := json.Unmarshal(result, &out.AccountingResult); err != nil {
			return nil, TrialBalanceOutput{}, fmt.Errorf(
				"decoding accounting result: %w",
				err,
			)
		}
	}

	return nil, out, nil
}

// ListChartOfAccounts implements the list_chart_of_accounts tool.
func (h *ExpertHandler) ListChartOfAccounts(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ChartOfAccountsInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	filters := map[string]string{
		"account_number":         in.AccountNumber,
		"pcg_entry_has_movement": boolParam(in.HasMovement),
	}
	if in.AccountingPeriodID != 0 {
		filters["accounting_period"] = strconv.FormatInt(in.AccountingPeriodID, 10)
	}

	accounts, err := h.api.ChartOfAccounts(ctx, companyID, filters)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(accounts), nil, nil
}

// ListJournals implements the list_journals tool.
func (h *ExpertHandler) ListJournals(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in JournalsInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if err := checkEnum("type", in.Type, journalTypes); err != nil {
		return nil, nil, err
	}

	journals, err := h.api.Journals(ctx, companyID, in.Type)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(journals), nil, nil
}

// ListCompanyLabels implements the list_company_labels tool.
func (h *ExpertHandler) ListCompanyLabels(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CompanyLabelsInput,
) (*mcp.CallToolResult, tiime.RawPage, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, tiime.RawPage{}, err
	}

	filters := map[string]string{
		"account_number_prefixes": strings.Join(in.AccountRoots, "|"),
		"disabled":                boolParam(in.Disabled),
	}
	page, err := h.api.CompanyLabels(ctx, companyID, tiime.ExpertQuery{Search: in.Search, Filters: filters}, in.window())

	return nil, page, err
}

// ValidateMovements implements the validate_movements tool.
func (h *ExpertHandler) ValidateMovements(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ValidateMovementsInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if len(in.MovementIDs) == 0 {
		return nil, nil, errors.New("movement_ids is required: see list_movements")
	}

	movements, err := h.api.SetMovementsValidated(ctx, companyID, in.MovementIDs, in.Validated)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(movements), nil, nil
}

// ListClientBankTransactions implements the list_client_bank_transactions tool.
func (h *ExpertHandler) ListClientBankTransactions(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ClientTransactionsInput,
) (*mcp.CallToolResult, tiime.RawPage, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, tiime.RawPage{}, err
	}
	if err := checkDate(in.From); err != nil {
		return nil, tiime.RawPage{}, err
	}

	filters := map[string]string{}
	if in.From != "" {
		filters["start_date"] = ">=" + in.From
	}
	if in.BankAccountID != 0 {
		filters["bank_account"] = strconv.FormatInt(in.BankAccountID, 10)
	}
	if in.LabelID != 0 {
		filters["label_id"] = strconv.FormatInt(in.LabelID, 10)
	}

	query := tiime.ExpertQuery{Search: in.Search, Sort: "transaction_date:desc", Filters: filters}
	page, err := h.api.CompanyBankTransactions(ctx, companyID, query, in.window())

	return nil, page, err
}

// RequestReceipts implements the request_receipts tool.
func (h *ExpertHandler) RequestReceipts(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ReceiptRequestInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if len(in.TransactionIDs) == 0 {
		return nil, nil, errors.New("transaction_ids is required: see list_client_bank_transactions")
	}

	requests, err := h.api.RequestReceipts(ctx, companyID, in.TransactionIDs)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(requests), nil, nil
}

// CancelReceiptRequests implements the cancel_receipt_requests tool.
func (h *ExpertHandler) CancelReceiptRequests(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CancelReceiptRequestInput,
) (*mcp.CallToolResult, DoneOutput, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, DoneOutput{}, err
	}
	if len(in.RequestIDs) == 0 {
		return nil, DoneOutput{}, errors.New("request_ids is required: see accountant_detail_requests in list_client_bank_transactions")
	}

	if err := h.api.CancelReceiptRequests(ctx, companyID, in.RequestIDs); err != nil {
		return nil, DoneOutput{}, err
	}

	return nil, DoneOutput{Done: true}, nil
}
