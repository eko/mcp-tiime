package tools

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcp-tiime/internal/tiime"
)

func TestSummarize(
	t *testing.T,
) {
	transactions := []tiime.BankTransaction{
		{TransactionDate: "2026-09-29", Amount: -8000, Wording: "Salary"},
		{TransactionDate: "2026-10-01", Amount: 38.17, Wording: "Stripe"},
		{TransactionDate: "2026-10-01", Amount: -13.2, Wording: "Qonto"},
		{TransactionDate: "2026-10-20", Amount: -7411, Wording: "URSSAF"},
		{TransactionDate: "2026-11-02", Amount: 21371.33, Wording: "Malt"},
	}
	from := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

	testCases := []struct {
		name        string
		granularity string
		wantBuckets []CashBucket
		wantErr     bool
	}{
		{
			name:        "weeks start on monday",
			granularity: "week",
			wantBuckets: []CashBucket{
				{Start: "2026-09-28", End: "2026-10-04", CashFlow: CashFlow{Inflows: 38.17, Outflows: -8013.2, Net: -7975.03, Count: 3}},
				{Start: "2026-10-05", End: "2026-10-11"},
				{Start: "2026-10-12", End: "2026-10-18"},
				{Start: "2026-10-19", End: "2026-10-25", CashFlow: CashFlow{Outflows: -7411, Net: -7411, Count: 1}},
				{Start: "2026-10-26", End: "2026-10-27"},
			},
		},
		{
			name:        "months are clipped to the period",
			granularity: "month",
			wantBuckets: []CashBucket{
				{Start: "2026-09-28", End: "2026-09-30", CashFlow: CashFlow{Outflows: -8000, Net: -8000, Count: 1}},
				{Start: "2026-10-01", End: "2026-10-27", CashFlow: CashFlow{Inflows: 38.17, Outflows: -7424.2, Net: -7386.03, Count: 3}},
			},
		},
		{
			name:        "unknown granularity",
			granularity: "year",
			wantErr:     true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			output, err := summarize(transactions, from, to, testCase.granularity)
			if testCase.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.wantBuckets, output.Buckets)
			assert.Equal(t, CashFlow{Inflows: 38.17, Outflows: -15424.2, Net: -15386.03, Count: 4}, output.Total)
			require.Len(t, output.TopOutflows, 3)
			assert.Equal(t, "Salary", output.TopOutflows[0].Wording)
			require.Len(t, output.TopInflows, 1)
		})
	}
}

func TestSummaryPeriod(
	t *testing.T,
) {
	now := time.Date(2026, 10, 27, 15, 0, 0, 0, time.UTC)

	testCases := []struct {
		name     string
		from     string
		to       string
		wantFrom string
		wantTo   string
		wantErr  bool
	}{
		{name: "defaults to the last 30 days", wantFrom: "2026-09-27", wantTo: "2026-10-27"},
		{name: "explicit period", from: "2026-01-01", to: "2026-03-31", wantFrom: "2026-01-01", wantTo: "2026-03-31"},
		{name: "inverted period", from: "2026-03-31", to: "2026-01-01", wantErr: true},
		{name: "invalid date", from: "31/03/2026", wantErr: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			from, to, err := summaryPeriod(testCase.from, testCase.to, now)
			if testCase.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.wantFrom, from.Format(time.DateOnly))
			assert.Equal(t, testCase.wantTo, to.Format(time.DateOnly))
		})
	}
}
