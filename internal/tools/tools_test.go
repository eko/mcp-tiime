package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcp-tiime/internal/tiime"
)

type fakeAPI struct {
	// API panics on methods a test does not expect to be called.
	API
	companies     []tiime.Company
	companyCalls  int
	lastCompanyID int64
	lastFilter    tiime.DocumentFilter
	lastWindow    tiime.Range
	pdf           []byte
}

func (f *fakeAPI) Companies(
	_ context.Context,
) ([]tiime.Company, error) {
	f.companyCalls++
	return f.companies, nil
}

func (f *fakeAPI) Customers(
	_ context.Context,
	companyID int64,
	_ tiime.CustomerFilter,
	window tiime.Range,
) (tiime.Page[tiime.Customer], error) {
	f.lastCompanyID, f.lastWindow = companyID, window
	return tiime.Page[tiime.Customer]{Items: []tiime.Customer{{ID: 1, Name: "ACME"}}}, nil
}

func (f *fakeAPI) Customer(
	_ context.Context,
	companyID int64,
	customerID int64,
) (json.RawMessage, error) {
	f.lastCompanyID = companyID
	return json.RawMessage(`{"id":1,"name":"ACME"}`), nil
}

func (f *fakeAPI) Contacts(
	_ context.Context,
	_ int64,
	_ int64,
) ([]tiime.Contact, error) {
	return []tiime.Contact{{ID: 3, Email: "billing@acme.test"}}, nil
}

func (f *fakeAPI) Invoices(
	_ context.Context,
	companyID int64,
	filter tiime.DocumentFilter,
	window tiime.Range,
) (tiime.Page[tiime.Document], error) {
	f.lastCompanyID, f.lastFilter, f.lastWindow = companyID, filter, window
	return tiime.Page[tiime.Document]{Items: []tiime.Document{{ID: 7, Status: "paid"}}}, nil
}

func (f *fakeAPI) Invoice(
	_ context.Context,
	companyID int64,
	_ int64,
) (json.RawMessage, error) {
	f.lastCompanyID = companyID
	return json.RawMessage(`{"id":7,"lines":[]}`), nil
}

func (f *fakeAPI) InvoicePDF(
	_ context.Context,
	companyID int64,
	_ int64,
) ([]byte, error) {
	f.lastCompanyID = companyID
	return f.pdf, nil
}

func (f *fakeAPI) Quotes(
	ctx context.Context,
	companyID int64,
	filter tiime.DocumentFilter,
	window tiime.Range,
) (tiime.Page[tiime.Document], error) {
	return f.Invoices(ctx, companyID, filter, window)
}

func (f *fakeAPI) Quote(
	ctx context.Context,
	companyID int64,
	quoteID int64,
) (json.RawMessage, error) {
	return f.Invoice(ctx, companyID, quoteID)
}

func connect(
	t *testing.T,
	handler *Handler,
) *mcp.ClientSession {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	Register(server, handler)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	return session
}

func TestRegisterListsTools(
	t *testing.T,
) {
	session := connect(t, NewHandler(&fakeAPI{}, nil, 1, t.TempDir()))

	result, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	assert.ElementsMatch(t, []string{
		"annotate_bank_transactions",
		"answer_document_request",
		"cancel_invoice",
		"cancel_invoice_payment",
		"cash_summary",
		"complete_login",
		"create_client",
		"create_credit_note",
		"create_document_category",
		"create_expense_report",
		"create_invoice_draft",
		"create_invoice_from_quote",
		"create_label",
		"create_label_rule",
		"create_mileage_report",
		"create_quote",
		"create_travel",
		"declare_document_unavailable",
		"delete_article",
		"delete_document",
		"delete_document_category",
		"delete_expense_report",
		"delete_forecast_row",
		"delete_invoice_draft",
		"delete_label_rule",
		"delete_quote",
		"delete_tag",
		"download_document",
		"download_invoice_pdf",
		"duplicate_invoice",
		"duplicate_quote",
		"estimate_travel_amount",
		"export_revenue_book",
		"finalize_invoice",
		"get_bank_transaction",
		"get_client",
		"get_dashboard",
		"get_expense_report",
		"get_forecast",
		"get_invoice",
		"get_quote",
		"get_thread",
		"get_vat_details",
		"get_vat_estimate",
		"link_documents_to_transaction",
		"link_invoices_to_transaction",
		"list_accountant_requests",
		"list_articles",
		"list_bank_accounts",
		"list_bank_transactions",
		"list_clients",
		"list_companies",
		"list_document_categories",
		"list_documents",
		"list_einvoices",
		"list_expense_reports",
		"list_invoice_payments",
		"list_invoice_reminders",
		"list_invoices",
		"list_invoicing_units",
		"list_label_rules",
		"list_labels",
		"list_quotes",
		"list_revenue_book",
		"list_standard_labels",
		"list_tags",
		"list_threads",
		"list_transaction_documents",
		"list_travels",
		"list_vat_types",
		"list_vehicles",
		"mark_thread_read",
		"post_thread_message",
		"record_invoice_payment",
		"save_article",
		"save_forecast_row",
		"send_invoice",
		"send_invoice_reminder",
		"send_quote",
		"set_forecast_enabled",
		"start_login",
		"suggest_labels",
		"unlink_documents_from_transaction",
		"unlink_invoices_from_transaction",
		"update_client",
		"update_document",
		"update_expense_report",
		"update_invoice_draft",
		"update_label",
		"update_quote",
		"update_tag",
		"upload_document",
	}, names)
}

func TestCompanyResolution(
	t *testing.T,
) {
	testCases := []struct {
		name            string
		defaultID       int64
		companies       []tiime.Company
		arguments       map[string]any
		wantCompanyID   int64
		wantError       bool
		wantLookupCalls int
	}{
		{
			name:          "explicit company",
			defaultID:     1,
			arguments:     map[string]any{"company_id": 5},
			wantCompanyID: 5,
		},
		{
			name:          "configured default",
			defaultID:     1,
			arguments:     map[string]any{},
			wantCompanyID: 1,
		},
		{
			name:            "single company detected",
			companies:       []tiime.Company{{ID: 9, Name: "Solo"}},
			arguments:       map[string]any{},
			wantCompanyID:   9,
			wantLookupCalls: 1,
		},
		{
			name:            "ambiguous companies",
			companies:       []tiime.Company{{ID: 9, Name: "A"}, {ID: 10, Name: "B"}},
			arguments:       map[string]any{},
			wantError:       true,
			wantLookupCalls: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			api := &fakeAPI{companies: testCase.companies}
			session := connect(t, NewHandler(api, nil, testCase.defaultID, t.TempDir()))

			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "list_invoices",
				Arguments: testCase.arguments,
			})

			require.NoError(t, err)
			assert.Equal(t, testCase.wantError, result.IsError)
			assert.Equal(t, testCase.wantLookupCalls, api.companyCalls)
			if !testCase.wantError {
				assert.Equal(t, testCase.wantCompanyID, api.lastCompanyID)
			}
		})
	}
}

func TestListInvoicesArguments(
	t *testing.T,
) {
	testCases := []struct {
		name       string
		arguments  map[string]any
		wantFilter tiime.DocumentFilter
		wantWindow tiime.Range
	}{
		{
			name:       "defaults",
			arguments:  map[string]any{},
			wantWindow: tiime.Range{},
		},
		{
			name: "filters and capped limit",
			arguments: map[string]any{
				"status":        "draft",
				"emission_date": ">=2026-01-01",
				"offset":        10,
				"limit":         500,
			},
			wantFilter: tiime.DocumentFilter{Status: "draft", EmissionDate: ">=2026-01-01"},
			wantWindow: tiime.Range{Offset: 10, Limit: maxLimit},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			api := &fakeAPI{}
			session := connect(t, NewHandler(api, nil, 1, t.TempDir()))

			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "list_invoices",
				Arguments: testCase.arguments,
			})

			require.NoError(t, err)
			require.False(t, result.IsError)
			assert.Equal(t, testCase.wantFilter, api.lastFilter)
			assert.Equal(t, testCase.wantWindow, api.lastWindow)
		})
	}
}

func TestGetTools(
	t *testing.T,
) {
	testCases := []struct {
		name     string
		tool     string
		wantJSON string
	}{
		{
			name:     "invoice detail is passed through",
			tool:     "get_invoice",
			wantJSON: `{"id":7,"lines":[]}`,
		},
		{
			name:     "client detail includes contacts",
			tool:     "get_client",
			wantJSON: `{"client":{"id":1,"name":"ACME"},"contacts":[{"id":3,"email":"billing@acme.test"}]}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			session := connect(t, NewHandler(&fakeAPI{}, nil, 1, t.TempDir()))

			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      testCase.tool,
				Arguments: map[string]any{"id": 7},
			})

			require.NoError(t, err)
			require.False(t, result.IsError)
			require.Len(t, result.Content, 1)
			text, ok := result.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			assert.JSONEq(t, testCase.wantJSON, text.Text)
		})
	}
}

func TestDownloadInvoicePDF(
	t *testing.T,
) {
	dir := filepath.Join(t.TempDir(), "pdf")
	api := &fakeAPI{pdf: []byte("%PDF-1.7")}
	session := connect(t, NewHandler(api, nil, 1, dir))

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "download_invoice_pdf",
		Arguments: map[string]any{"id": 7},
	})

	require.NoError(t, err)
	require.False(t, result.IsError)

	content, err := os.ReadFile(filepath.Join(dir, "tiime-invoice-7.pdf"))
	require.NoError(t, err)
	assert.Equal(t, "%PDF-1.7", string(content))
}
