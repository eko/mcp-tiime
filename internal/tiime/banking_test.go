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

func TestClientBankTransactions(
	t *testing.T,
) {
	unmatched := false

	testCases := []struct {
		name      string
		filter    TransactionFilter
		wantQuery string
	}{
		{
			name:      "no filter",
			wantQuery: "",
		},
		{
			name:      "unmatched transactions with search",
			filter:    TransactionFilter{Search: "urssaf", Matched: &unmatched},
			wantQuery: "matched=false&q=urssaf",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/accounts/companies/42/bank_transactions", r.URL.Path)
				assert.Equal(t, testCase.wantQuery, r.URL.RawQuery)
				assert.Equal(t, "application/vnd.tiime.bank_transactions.v2+json", r.Header.Get("Accept"))
				_, _ = w.Write([]byte(`{"transactions":[{"id":1,"amount":-420,"count_documents":1,"count_invoices":0,
					"imputations":[{"id":2,"amount":-420,"count_documents":0}]}],"metadata":{"total_amount":-420}}`))
			}))
			defer server.Close()

			client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

			page, err := client.BankTransactions(context.Background(), 42, testCase.filter, Range{})

			require.NoError(t, err)
			require.Len(t, page.Items, 1)
			assert.Equal(t, 1, page.Items[0].CountDocuments)
		})
	}
}

func TestAnnotateTransactions(
	t *testing.T,
) {
	empty := ""
	comment := "Invoice to come"

	testCases := []struct {
		name        string
		annotation  TransactionAnnotation
		wantUpdates []map[string]any
	}{
		{
			name:        "comment",
			annotation:  TransactionAnnotation{Comment: &comment},
			wantUpdates: []map[string]any{{"comment": "Invoice to come"}},
		},
		{
			name:        "empty comment is cleared with null",
			annotation:  TransactionAnnotation{Comment: &empty},
			wantUpdates: []map[string]any{{"comment": nil}},
		},
		{
			name:       "label and tags are sent separately",
			annotation: TransactionAnnotation{LabelID: 5, Tags: []string{"Qonto"}},
			wantUpdates: []map[string]any{
				{"imputations": []any{map[string]any{"label": map[string]any{"id": float64(5)}}}},
				{"tags": []any{map[string]any{"name": "Qonto"}}},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var updates []map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPatch, r.Method)
				var body struct {
					IDs    []int64        `json:"bank_transaction_ids"`
					Update map[string]any `json:"bank_transaction"`
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, []int64{7}, body.IDs)
				updates = append(updates, body.Update)
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

			require.NoError(t, client.AnnotateTransactions(context.Background(), 42, []int64{7}, testCase.annotation))
			assert.Equal(t, testCase.wantUpdates, updates)
		})
	}
}
