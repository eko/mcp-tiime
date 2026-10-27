package tools

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// maxSummaryTransactions bounds the transactions loaded for a cash summary.
const maxSummaryTransactions = 2000

// CashSummaryInput selects the period of a cash summary.
type CashSummaryInput struct {
	CompanyInput
	From          string `json:"from,omitempty" jsonschema:"First day, YYYY-MM-DD, defaults to 30 days before to"`
	To            string `json:"to,omitempty" jsonschema:"Last day, YYYY-MM-DD, defaults to today"`
	Granularity   string `json:"granularity,omitempty" jsonschema:"day, week (default) or month"`
	BankAccountID int64  `json:"bank_account_id,omitempty" jsonschema:"Only this bank account, see list_bank_accounts"`
}

// CashFlow sums inflows and outflows.
type CashFlow struct {
	Inflows  float64 `json:"inflows"`
	Outflows float64 `json:"outflows"`
	Net      float64 `json:"net"`
	Count    int     `json:"count"`
}

// CashBucket is the cash flow of a sub period.
type CashBucket struct {
	Start string `json:"start"`
	End   string `json:"end"`
	CashFlow
}

// CashMovement is a notable transaction of the period.
type CashMovement struct {
	Date    string  `json:"date"`
	Wording string  `json:"wording"`
	Amount  float64 `json:"amount"`
}

// CashSummaryOutput summarizes the bank movements of a period.
type CashSummaryOutput struct {
	From        string         `json:"from"`
	To          string         `json:"to"`
	Total       CashFlow       `json:"total"`
	Buckets     []CashBucket   `json:"buckets"`
	TopInflows  []CashMovement `json:"top_inflows"`
	TopOutflows []CashMovement `json:"top_outflows"`
	Truncated   bool           `json:"truncated,omitempty"`
}

func registerCashflow(
	server *mcp.Server,
	h *Handler,
) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cash_summary",
		Description: "Summarize bank inflows and outflows over a period, by day, week or month, with the largest movements.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, h.CashSummary)
}

// CashSummary implements the cash_summary tool.
func (h *Handler) CashSummary(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CashSummaryInput,
) (*mcp.CallToolResult, CashSummaryOutput, error) {
	from, to, err := summaryPeriod(in.From, in.To, time.Now())
	if err != nil {
		return nil, CashSummaryOutput{}, err
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, CashSummaryOutput{}, err
	}

	filter := tiime.TransactionFilter{
		BankAccountID: in.BankAccountID,
		Date:          ">=" + from.Format(time.DateOnly) + ",<=" + to.Format(time.DateOnly),
	}
	transactions, truncated, err := h.api.AllBankTransactions(ctx, companyID, filter, maxSummaryTransactions)
	if err != nil {
		return nil, CashSummaryOutput{}, err
	}

	output, err := summarize(transactions, from, to, in.Granularity)
	output.Truncated = truncated

	return nil, output, err
}

func summaryPeriod(
	fromInput string,
	toInput string,
	now time.Time,
) (time.Time, time.Time, error) {
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if toInput != "" {
		parsed, err := time.Parse(time.DateOnly, toInput)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid to date %q: %w", toInput, err)
		}
		to = parsed
	}

	from := to.AddDate(0, 0, -30)
	if fromInput != "" {
		parsed, err := time.Parse(time.DateOnly, fromInput)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid from date %q: %w", fromInput, err)
		}
		from = parsed
	}

	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("from %s is after to %s", from.Format(time.DateOnly), to.Format(time.DateOnly))
	}

	return from, to, nil
}

// summarize groups transactions into buckets of the given granularity.
func summarize(
	transactions []tiime.BankTransaction,
	from time.Time,
	to time.Time,
	granularity string,
) (CashSummaryOutput, error) {
	bucketStart, err := bucketer(granularity)
	if err != nil {
		return CashSummaryOutput{}, err
	}

	var buckets []CashBucket
	index := map[string]int{}
	for day := bucketStart(from); !day.After(to); day = nextBucket(day, granularity) {
		end := nextBucket(day, granularity).AddDate(0, 0, -1)
		start := day
		if start.Before(from) {
			start = from
		}
		if end.After(to) {
			end = to
		}
		index[day.Format(time.DateOnly)] = len(buckets)
		buckets = append(buckets, CashBucket{Start: start.Format(time.DateOnly), End: end.Format(time.DateOnly)})
	}

	output := CashSummaryOutput{
		From:    from.Format(time.DateOnly),
		To:      to.Format(time.DateOnly),
		Buckets: buckets,
	}

	var movements []CashMovement
	for _, transaction := range transactions {
		date, err := time.Parse(time.DateOnly, transaction.TransactionDate)
		if err != nil || date.Before(from) || date.After(to) {
			continue
		}

		i := index[bucketStart(date).Format(time.DateOnly)]
		output.Buckets[i].add(transaction.Amount)
		output.Total.add(transaction.Amount)
		movements = append(movements, CashMovement{Date: transaction.TransactionDate, Wording: transaction.Wording, Amount: transaction.Amount})
	}

	output.Total.round()
	for i := range output.Buckets {
		output.Buckets[i].round()
	}

	sort.SliceStable(movements, func(i, j int) bool { return movements[i].Amount > movements[j].Amount })
	for _, movement := range movements {
		if movement.Amount > 0 && len(output.TopInflows) < 5 {
			output.TopInflows = append(output.TopInflows, movement)
		}
	}
	for i := len(movements) - 1; i >= 0 && len(output.TopOutflows) < 5; i-- {
		if movements[i].Amount < 0 {
			output.TopOutflows = append(output.TopOutflows, movements[i])
		}
	}

	return output, nil
}

func (f *CashFlow) add(
	amount float64,
) {
	if amount >= 0 {
		f.Inflows += amount
	} else {
		f.Outflows += amount
	}
	f.Net += amount
	f.Count++
}

func (f *CashFlow) round() {
	f.Inflows = roundCents(f.Inflows)
	f.Outflows = roundCents(f.Outflows)
	f.Net = roundCents(f.Net)
}

func roundCents(
	value float64,
) float64 {
	return math.Round(value*100) / 100
}

// bucketer returns the function giving the first day of the bucket of a day.
func bucketer(
	granularity string,
) (func(time.Time) time.Time, error) {
	switch granularity {
	case "day":
		return func(day time.Time) time.Time { return day }, nil
	case "", "week":
		return func(day time.Time) time.Time {
			// Weeks start on Monday.
			offset := (int(day.Weekday()) + 6) % 7
			return day.AddDate(0, 0, -offset)
		}, nil
	case "month":
		return func(day time.Time) time.Time {
			return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
		}, nil
	default:
		return nil, fmt.Errorf("unknown granularity %q, expected day, week or month", granularity)
	}
}

func nextBucket(
	start time.Time,
	granularity string,
) time.Time {
	switch granularity {
	case "day":
		return start.AddDate(0, 0, 1)
	case "month":
		return start.AddDate(0, 1, 0)
	default:
		return start.AddDate(0, 0, 7)
	}
}
