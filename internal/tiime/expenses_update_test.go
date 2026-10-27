package tiime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateExpenseReport(
	t *testing.T,
) {
	var patched map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("GET /accounts/companies/42/expense_reports/5", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":5,"name":"Expenses 11/2026","date":"2026-11-01","expense_type":"advanced_expense",
			"lifecycle_status":"draft","owner":{"id":1,"firstname":"V"},"preview_available":true,
			"advanced_expenses":[{"id":10,"amount":31.99,"currency":"EUR","date":"2026-11-02","preview_available":true},
			{"id":11,"amount":15.99,"currency":"EUR","date":"2026-11-07"}]}`))
	})
	mux.HandleFunc("GET /accounts/companies/42/documents/12", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":12,"name":"IBIS","metadata":[{"key":"date","value":"2026-11-10 00:00:00"},
			{"key":"amount","value":{"value":120,"currency":"EUR"}},{"key":"vat_amount","type":"amounts","value":[{"key":"total","value":{"value":10.91}}]}]}`))
	})
	mux.HandleFunc("PATCH /accounts/companies/42/expense_reports/5", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "preview_available", r.URL.Query().Get("expand"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&patched))
		_, _ = w.Write([]byte(`{"id":5}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

	_, err := client.UpdateExpenseReport(context.Background(), 42, 5, ExpenseReportUpdate{
		AddDocumentIDs:    []int64{12},
		RemoveDocumentIDs: []int64{11},
		Submit:            true,
	})

	require.NoError(t, err)
	assert.Equal(t, "saved", patched["lifecycle_status"])
	assert.Equal(t, "Expenses 11/2026", patched["name"])
	assert.NotContains(t, patched, "preview_available")

	expenses, ok := patched["advanced_expenses"].([]any)
	require.True(t, ok)
	require.Len(t, expenses, 2)
	assert.InDelta(t, 10.0, expenses[0].(map[string]any)["id"], 0.1)
	assert.NotContains(t, expenses[0], "preview_available")
	added := expenses[1].(map[string]any)
	assert.InDelta(t, 12.0, added["id"], 0.1)
	assert.InDelta(t, 120.0, added["amount"], 0.001)
	assert.InDelta(t, 10.91, added["vat_amount"], 0.001)
	assert.Equal(t, "2026-11-10 00:00:00", added["date"])
}

func TestAccountingDocumentSummarize(
	t *testing.T,
) {
	testCases := []struct {
		name    string
		raw     string
		wantVAT *float64
	}{
		{
			name:    "single VAT amount",
			raw:     `{"metadata":[{"key":"vat_amount","value":{"value":2.67}}]}`,
			wantVAT: ptr(2.67),
		},
		{
			name:    "VAT by rate",
			raw:     `{"metadata":[{"key":"vat_amount","value":[{"key":"total","value":{"value":3561.89}}]}]}`,
			wantVAT: ptr(3561.89),
		},
		{
			name: "VAT by rate without value",
			raw:  `{"metadata":[{"key":"vat_amount","value":[{"key":"total","value":null}]}]}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var document AccountingDocument
			require.NoError(t, json.Unmarshal([]byte(testCase.raw), &document))

			document.Summarize()

			assert.Equal(t, testCase.wantVAT, document.VATAmount)
		})
	}
}

func ptr(
	value float64,
) *float64 {
	return &value
}
