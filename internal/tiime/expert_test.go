package tiime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expertCall is an expected request of the expert application.
type expertCall struct {
	method    string
	path      string
	query     string
	rangeHdr  string
	body      string
	headers   map[string]string
	response  string
	noContent bool
}

func TestExpertClient(
	t *testing.T,
) {
	ctx := context.Background()

	testCases := []struct {
		name string
		want expertCall
		call func(c *Client) error
	}{
		{
			name: "lists the portfolio with filters",
			want: expertCall{
				method:   http.MethodGet,
				path:     "/expert/companies",
				query:    "business_units=1%2C2&q=acme&sorts=name%3Aasc",
				rangeHdr: "items=0-49",
				response: `[{"id":7,"name":"ACME"}]`,
			},
			call: func(c *Client) error {
				page, err := c.ExpertCompanies(ctx, ExpertQuery{Search: "acme", Sort: "name:asc", Filters: map[string]string{"business_units": "1,2", "empty": ""}}, Range{})
				if err == nil {
					assert.Equal(t, "ACME", page.Items[0]["name"])
				}
				return err
			},
		},
		{
			name: "reads a company with expand",
			want: expertCall{method: http.MethodGet, path: "/expert/companies/7", query: "expand=legal_company", response: `{"id":7}`},
			call: func(c *Client) error {
				_, err := c.ExpertCompany(ctx, 7, "legal_company")
				return err
			},
		},
		{
			name: "lists the tasks of a company without pagination",
			want: expertCall{method: http.MethodGet, path: "/expert/users/me/tasks", query: "company=7", response: `[{"id":1}]`},
			call: func(c *Client) error {
				tasks, err := c.ExpertCompanyTasks(ctx, 7)
				if err == nil {
					assert.Len(t, tasks, 1)
				}
				return err
			},
		},
		{
			name: "sums the tasks without sort",
			want: expertCall{method: http.MethodGet, path: "/expert/users/me/tasks/totals", query: "status=done", response: `{"amount":10}`},
			call: func(c *Client) error {
				_, err := c.ExpertTaskTotals(ctx, ExpertQuery{Sort: "deadline_date:asc", Filters: map[string]string{"status": "done"}})
				return err
			},
		},
		{
			name: "creates a task",
			want: expertCall{method: http.MethodPost, path: "/expert/tasks", body: `{"company":{"id":7}}`, response: `{"id":1}`},
			call: func(c *Client) error {
				_, err := c.CreateExpertTask(ctx, map[string]any{"company": map[string]any{"id": 7}})
				return err
			},
		},
		{
			name: "updates the next occurrences of a task",
			want: expertCall{method: http.MethodPatch, path: "/expert/companies/7/tasks/1", query: "apply_nexts=2026-11-30", body: `{"status":"ignored"}`, response: `{"id":1}`},
			call: func(c *Client) error {
				_, err := c.UpdateExpertTask(ctx, 7, 1, map[string]any{"status": "ignored"}, "2026-11-30")
				return err
			},
		},
		{
			name: "deletes a task",
			want: expertCall{method: http.MethodDelete, path: "/expert/companies/7/tasks/1", noContent: true},
			call: func(c *Client) error {
				return c.DeleteExpertTask(ctx, 7, 1, "")
			},
		},
		{
			name: "removes the billing mark of a task",
			want: expertCall{method: http.MethodPost, path: "/expert/companies/7/tasks/1/billing_unpoint", body: `{}`, response: `{"id":1}`},
			call: func(c *Client) error {
				_, err := c.SetExpertTaskBillingPointed(ctx, 7, 1, false)
				return err
			},
		},
		{
			name: "saves a time record with PUT on the collection",
			want: expertCall{method: http.MethodPut, path: "/expert/time_tracking/records", body: `{"duration":90}`, response: `{"id":3}`},
			call: func(c *Client) error {
				_, err := c.SaveTimeRecord(ctx, map[string]any{"duration": 90})
				return err
			},
		},
		{
			name: "validates time records",
			want: expertCall{
				method:    http.MethodPost,
				path:      "/expert/time_tracking/records/validate",
				body:      `{"end_date":"2026-10-31","start_date":"2026-10-27","user_ids":[5],"validated":true}`,
				noContent: true,
			},
			call: func(c *Client) error {
				return c.ValidateTimeRecords(ctx, []int64{5}, "2026-10-27", "2026-10-31", true)
			},
		},
		{
			name: "lists movements with the expansion of the expert application",
			want: expertCall{
				method:   http.MethodGet,
				path:     "/expert/companies/7/movements",
				query:    "date=%3E%3D2026-01-01%2C%3C%3D2026-12-31&expand=lettering%2Caccounting_entry_counterpart%2Cvat_amount%2Chas_attachment&pcg_entry=12",
				rangeHdr: "items=0-49",
				response: `[]`,
			},
			call: func(c *Client) error {
				_, err := c.Movements(ctx, 7, ExpertQuery{Filters: map[string]string{"date": ">=2026-01-01,<=2026-12-31", "pcg_entry": "12"}}, Range{})
				return err
			},
		},
		{
			name: "validates movements",
			want: expertCall{method: http.MethodPost, path: "/expert/companies/7/movements/validate", body: `{"movements":[{"id":1},{"id":2}]}`, response: `[]`},
			call: func(c *Client) error {
				_, err := c.SetMovementsValidated(ctx, 7, []int64{1, 2}, true)
				return err
			},
		},
		{
			name: "reads the trial balance of a period",
			want: expertCall{method: http.MethodGet, path: "/expert/companies/7/accounting_periods/3/balances", query: "amount=%3C0", rangeHdr: "items=0-49", response: `[]`},
			call: func(c *Client) error {
				_, err := c.TrialBalance(ctx, 7, 3, ExpertQuery{Filters: map[string]string{"amount": "<0"}}, Range{})
				return err
			},
		},
		{
			name: "lists client bank transactions with the v2 representation",
			want: expertCall{
				method:   http.MethodGet,
				path:     "/expert/companies/7/bank_transactions",
				query:    "expand=imputations&sorts=transaction_date%3Adesc",
				rangeHdr: "items=0-49",
				headers:  map[string]string{"Accept": "application/vnd.tiime.bank_transactions.v2+json"},
				response: `[]`,
			},
			call: func(c *Client) error {
				_, err := c.CompanyBankTransactions(ctx, 7, ExpertQuery{Sort: "transaction_date:desc"}, Range{})
				return err
			},
		},
		{
			name: "requests receipts in bulk",
			want: expertCall{
				method:   http.MethodPost,
				path:     "/expert/companies/7/bank_transactions/accountant_detail_requests",
				body:     `{"type":"receipt","bank_transactions":[{"id":5}]}`,
				response: `[{"id":9,"type":"receipt"}]`,
			},
			call: func(c *Client) error {
				_, err := c.RequestReceipts(ctx, 7, []int64{5})
				return err
			},
		},
		{
			name: "cancels receipt requests with a DELETE body",
			want: expertCall{
				method:    http.MethodDelete,
				path:      "/expert/companies/7/bank_transactions/accountant_detail_requests",
				body:      `{"type":"receipt","accountant_detail_requests":[{"id":9}]}`,
				noContent: true,
			},
			call: func(c *Client) error {
				return c.CancelReceiptRequests(ctx, 7, []int64{9})
			},
		},
		{
			name: "lists the VAT planning with its counter header",
			want: expertCall{
				method:   http.MethodGet,
				path:     "/expert/users/me/vat_declarations",
				query:    "expand=accounting_config&vat_declaration_status=draft",
				rangeHdr: "items=0-49",
				headers:  map[string]string{"X-Total-Not-Configured-Company": "true"},
				response: `[]`,
			},
			call: func(c *Client) error {
				_, err := c.VATPlanning(ctx, ExpertQuery{Filters: map[string]string{"vat_declaration_status": "draft"}}, Range{})
				return err
			},
		},
		{
			name: "reads VAT consistency checks with the v4 representation",
			want: expertCall{
				method:   http.MethodGet,
				path:     "/expert/companies/7/vat_declarations/4/consistency_check",
				headers:  map[string]string{"Accept": "application/vnd.tiime.consistency_checks.v4+json"},
				response: `[]`,
			},
			call: func(c *Client) error {
				_, err := c.VATConsistencyChecks(ctx, 7, 4)
				return err
			},
		},
		{
			name: "downloads a VAT declaration with a POST",
			want: expertCall{method: http.MethodPost, path: "/expert/companies/7/vat_declarations/4/pdf", body: `{"type":"vat_declaration"}`, response: "%PDF"},
			call: func(c *Client) error {
				content, _, err := c.VATDeclarationFile(ctx, 7, 4, "vat_declaration")
				assert.Equal(t, "%PDF", string(content))
				return err
			},
		},
		{
			name: "lists the client indicators with every indicator",
			want: expertCall{
				method:   http.MethodGet,
				path:     "/expert/company_accounting_indicators",
				query:    "expand=" + url.QueryEscape(indicatorsExpand) + "&valid=false",
				rangeHdr: "items=0-49",
				response: `[]`,
			},
			call: func(c *Client) error {
				_, err := c.ClientIndicators(ctx, ExpertQuery{Filters: map[string]string{"valid": "false"}}, Range{})
				return err
			},
		},
		{
			name: "marks a thread as read without body",
			want: expertCall{method: http.MethodPost, path: "/expert/companies/7/threads/8/read", noContent: true},
			call: func(c *Client) error {
				return c.ExpertThreadAction(ctx, 7, 8, "read")
			},
		},
		{
			name: "marks a notification as read",
			want: expertCall{method: http.MethodPatch, path: "/expert/users/me/notifications/6", body: `{"read":true}`, response: `{"id":6}`},
			call: func(c *Client) error {
				return c.MarkNotificationRead(ctx, 6)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := expertServer(t, testCase.want)
			defer server.Close()

			client := NewClient(server.URL, ExpertApp.Version, &staticTokens{tokens: []string{"t"}}, server.Client(), WithAppHeader(ExpertApp.Header))

			require.NoError(t, testCase.call(client))
		})
	}
}

// expertServer checks a single request of the expert application.
func expertServer(
	t *testing.T,
	want expertCall,
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, want.method, r.Method)
		assert.Equal(t, want.path, r.URL.Path)
		assert.Equal(t, want.query, r.URL.RawQuery)
		assert.Equal(t, want.rangeHdr, r.Header.Get("Range"))
		assert.Equal(t, "tiime-pro", r.Header.Get("tiime-app"))
		assert.Equal(t, ExpertApp.Version, r.Header.Get("tiime-app-version"))
		for key, value := range want.headers {
			assert.Equal(t, value, r.Header.Get(key), key)
		}

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		if want.body == "" {
			assert.Empty(t, body)
		} else {
			assert.JSONEq(t, want.body, string(body))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		}

		if want.noContent {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(want.response))
	}))
}

func TestExpertQueryDropsEmptyFilters(
	t *testing.T,
) {
	values := ExpertQuery{Filters: map[string]string{"a": "", "b": "1"}}.values()

	encoded, err := json.Marshal(values)
	require.NoError(t, err)
	assert.JSONEq(t, `{"b":["1"]}`, string(encoded))
}
