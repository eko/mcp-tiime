package tiime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientExpenseReports(
	t *testing.T,
) {
	testCases := []struct {
		name          string
		body          string
		wantAmount    float64
		wantVATAmount float64
	}{
		{
			name: "single VAT amount",
			body: `{"expense_reports":[{"id":1,"name":"Expenses 10/2026","metadata":[
				{"key":"amount","type":"amount","value":{"currency":"EUR","value":574.57}},
				{"key":"vat_amount","type":"amount","value":{"currency":"EUR","value":95.76}}]}],"metadata":{}}`,
			wantAmount:    574.57,
			wantVATAmount: 95.76,
		},
		{
			name: "VAT split by rate",
			body: `{"expense_reports":[{"id":1,"metadata":[
				{"key":"amount","type":"amount","value":{"value":12}},
				{"key":"vat_amount","type":"amounts","value":[{"key":"total","value":{"value":2}}]}]}]}`,
			wantAmount:    12,
			wantVATAmount: 2,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/accounts/companies/42/expense_reports", r.URL.Path)
				assert.Equal(t, "date:desc", r.URL.Query().Get("sorts"))
				assert.Equal(t, "items=0-9", r.Header.Get("Range"))
				w.Header().Set("Content-Range", "items 0-9/*")
				_, _ = w.Write([]byte(testCase.body))
			}))
			defer server.Close()

			client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

			page, err := client.ExpenseReports(context.Background(), 42, Range{Limit: 10})

			require.NoError(t, err)
			require.Len(t, page.Items, 1)
			assert.InDelta(t, testCase.wantAmount, page.Items[0].Amount, 0.001)
			assert.InDelta(t, testCase.wantVATAmount, page.Items[0].VATAmount, 0.001)
			assert.Nil(t, page.Items[0].Metadata)
			assert.True(t, page.HasMore)
		})
	}
}
