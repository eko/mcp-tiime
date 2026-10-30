package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcp-tiime/internal/tiime"
)

type fakeExpertAPI struct {
	// ExpertAPI panics on methods a test does not expect to be called.
	ExpertAPI
	tasks      []json.RawMessage
	lastQuery  tiime.ExpertQuery
	lastBody   map[string]any
	lastNexts  string
	lastTaskID int64
}

func (f *fakeExpertAPI) ExpertMe(
	_ context.Context,
) (json.RawMessage, error) {
	return json.RawMessage(`{"id":5,"firstname":"Ada"}`), nil
}

func (f *fakeExpertAPI) ExpertCompanies(
	_ context.Context,
	query tiime.ExpertQuery,
	_ tiime.Range,
) (tiime.RawPage, error) {
	f.lastQuery = query
	return tiime.RawPage{Items: []map[string]any{{"id": float64(7), "name": "ACME"}}}, nil
}

func (f *fakeExpertAPI) ExpertTasks(
	_ context.Context,
	query tiime.ExpertQuery,
	_ tiime.Range,
) (tiime.RawPage, error) {
	f.lastQuery = query
	return tiime.RawPage{Items: []map[string]any{}}, nil
}

func (f *fakeExpertAPI) ExpertCompanyTasks(
	_ context.Context,
	_ int64,
) ([]json.RawMessage, error) {
	return f.tasks, nil
}

func (f *fakeExpertAPI) CreateExpertTask(
	_ context.Context,
	task map[string]any,
) (json.RawMessage, error) {
	f.lastBody = task
	return json.RawMessage(`{"id":1}`), nil
}

func (f *fakeExpertAPI) UpdateExpertTask(
	_ context.Context,
	_ int64,
	taskID int64,
	task map[string]any,
	applyNexts string,
) (json.RawMessage, error) {
	f.lastTaskID, f.lastBody, f.lastNexts = taskID, task, applyNexts
	return json.RawMessage(`{"id":1}`), nil
}

func (f *fakeExpertAPI) TimeTrackingReference(
	_ context.Context,
	kind string,
	_ map[string]string,
) (json.RawMessage, error) {
	switch kind {
	case "invoicing_types":
		return json.RawMessage(`[{"id":10,"default":false},{"id":11,"default":true}]`), nil
	case "tasks":
		return json.RawMessage(`[{"id":20,"time_tracking_category":{"id":30}}]`), nil
	default:
		return json.RawMessage(`[]`), nil
	}
}

func (f *fakeExpertAPI) SaveTimeRecord(
	_ context.Context,
	record map[string]any,
) (json.RawMessage, error) {
	f.lastBody = record
	return json.RawMessage(`{"id":3}`), nil
}

func (f *fakeExpertAPI) AccountingPeriods(
	_ context.Context,
	_ int64,
) (json.RawMessage, error) {
	return json.RawMessage(`[{"id":3,"start_date":"2026-01-01","end_date":"2026-12-31","closed":false}]`), nil
}

func (f *fakeExpertAPI) Movements(
	_ context.Context,
	_ int64,
	query tiime.ExpertQuery,
	_ tiime.Range,
) (tiime.RawPage, error) {
	f.lastQuery = query
	return tiime.RawPage{Items: []map[string]any{}}, nil
}

func (f *fakeExpertAPI) TrialBalance(
	_ context.Context,
	_ int64,
	_ int64,
	query tiime.ExpertQuery,
	_ tiime.Range,
) (tiime.RawPage, error) {
	f.lastQuery = query
	return tiime.RawPage{Items: []map[string]any{}}, nil
}

func (f *fakeExpertAPI) AccountingResult(
	_ context.Context,
	_ int64,
	_ int64,
) (json.RawMessage, error) {
	return json.RawMessage(`{"accounting_result":1234.5}`), nil
}

func (f *fakeExpertAPI) VATPlanning(
	_ context.Context,
	query tiime.ExpertQuery,
	_ tiime.Range,
) (tiime.RawPage, error) {
	f.lastQuery = query
	return tiime.RawPage{Items: []map[string]any{}}, nil
}

func (f *fakeExpertAPI) ExpertThreads(
	_ context.Context,
	_ int64,
	query tiime.ExpertQuery,
	_ tiime.Range,
) (tiime.RawPage, error) {
	f.lastQuery = query
	return tiime.RawPage{Items: []map[string]any{}}, nil
}

func (f *fakeExpertAPI) CreateExpertThread(
	_ context.Context,
	_ int64,
	thread map[string]any,
) (json.RawMessage, error) {
	f.lastBody = thread
	return json.RawMessage(`{"id":8}`), nil
}

func connectExpert(
	t *testing.T,
	handler *ExpertHandler,
) *mcp.ClientSession {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterExpert(server, handler)

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

// callExpert calls a tool and returns its error text, empty on success.
func callExpert(
	t *testing.T,
	session *mcp.ClientSession,
	name string,
	args map[string]any,
) string {
	t.Helper()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	if !result.IsError {
		return ""
	}

	return result.Content[0].(*mcp.TextContent).Text
}

// asJSON normalizes a request body for comparison.
func asJSON(
	t *testing.T,
	value any,
) string {
	t.Helper()

	encoded, err := json.Marshal(value)
	require.NoError(t, err)

	return string(encoded)
}

func TestExpertPortfolioTools(
	t *testing.T,
) {
	task := `{"id":1,"wording":"TVA","deadline_date":"2026-11-30","status":"draft","company":{"id":7,"name":"ACME"},
		"task_scheduler":{"id":9},"time_tracking_task":{"id":20,"wording":"TVA"},"time_tracking_invoicing_type":{"id":11,"wording":"Courante"},
		"responsable":{"id":5},"supervisor":null,"invoicing_status":"to_bill","amount":100,"discount":0,"threads":[],"locked":false}`

	testCases := []struct {
		name      string
		tool      string
		args      map[string]any
		wantErr   string
		wantQuery *tiime.ExpertQuery
		wantBody  string
		wantNexts string
	}{
		{
			name: "maps the active status and joins business units with commas",
			tool: "list_portfolio_companies",
			args: map[string]any{"status": "active", "business_unit_ids": []int64{1, 2}},
			wantQuery: &tiime.ExpertQuery{Filters: map[string]string{
				"mission_ended": "false", "in_creation_process": "false", "initialization_in_progress": "false",
				"mission_works_stopped": "false", "business_units": "1,2", "tax_regime_code": "", "contributor_ids": "",
			}},
		},
		{
			name:    "rejects an unknown company status",
			tool:    "list_portfolio_companies",
			args:    map[string]any{"status": "closed"},
			wantErr: `invalid status "closed"`,
		},
		{
			name: "lists tasks with the default statuses and sort",
			tool: "list_tasks",
			args: map[string]any{"deadline_from": "2026-11-01", "overdue": true},
			wantQuery: &tiime.ExpertQuery{Sort: "deadline_date:asc", Filters: map[string]string{
				"status":           "draft|waiting_client|to_check|to_correct|to_be_returned|to_supervise|done",
				"date":             ">=2026-11-01",
				"out_of_deadline":  "true",
				"invoicing_status": "", "billing_pointed": "", "responsable_ids": "", "supervisor_ids": "", "task_ids": "",
			}},
		},
		{
			name:    "rejects an invalid deadline",
			tool:    "list_tasks",
			args:    map[string]any{"deadline_to": "30/11/2026"},
			wantErr: `invalid date "30/11/2026"`,
		},
		{
			name: "creates a monthly task with the default responsable and invoicing type",
			tool: "create_task",
			args: map[string]any{
				"company_id": 7, "task_type_id": 20, "deadline_date": "2026-11-30", "wording": "TVA",
				"recurrence": map[string]any{"interval_months": 1, "occurrences": 12},
			},
			wantBody: `{"amount":null,"company":{"id":7},"deadline_date":"2026-11-30","discount":null,"invoicing_status":null,
				"responsable":{"id":5},"supervisor":null,"time_tracking_invoicing_type":{"id":11},"time_tracking_task":{"id":20},"wording":"TVA",
				"task_scheduler":{"amount":null,"company":{"id":7},"day":30,"discount":null,"first_deadline_date":"2026-11-30","interval":1,
				"invoicing_status":null,"last_deadline_date":null,"occurence":12,"responsable":{"id":5},"supervisor":null,
				"time_tracking_invoicing_type":{"id":11},"time_tracking_task":{"id":20},"wording":"TVA"}}`,
		},
		{
			name:    "requires one end for a recurrence",
			tool:    "create_task",
			args:    map[string]any{"company_id": 7, "task_type_id": 20, "deadline_date": "2026-11-30", "recurrence": map[string]any{"interval_months": 1}},
			wantErr: "either last_deadline_date or occurrences",
		},
		{
			name: "sends back the whole task with the changes",
			tool: "update_task",
			args: map[string]any{"company_id": 7, "task_id": 1, "status": "ignored", "apply_to_next_occurrences": true},
			wantBody: `{"amount":100,"company":{"id":7,"name":"ACME"},"deadline_date":"2026-11-30","discount":0,"id":1,"invoicing_status":"to_bill",
				"responsable":{"id":5},"status":"ignored","supervisor":null,"task_scheduler":{"id":9},"time_tracking_invoicing_type":{"id":11},
				"time_tracking_task":{"id":20},"wording":"TVA"}`,
			wantNexts: "2026-11-30",
		},
		{
			name:    "refuses a discount above the amount",
			tool:    "update_task",
			args:    map[string]any{"company_id": 7, "task_id": 1, "discount": 150},
			wantErr: "discount must not exceed amount",
		},
		{
			name:    "reports a task missing from the company",
			tool:    "update_task",
			args:    map[string]any{"company_id": 7, "task_id": 99, "status": "done"},
			wantErr: "task 99 not found",
		},
		{
			name: "records time with the category of the task type",
			tool: "save_time_record",
			args: map[string]any{"date": "2026-10-30", "task_type_id": 20, "duration_minutes": 90, "company_id": 7},
			wantBody: `{"accounting_period":null,"company":{"id":7},"date":"2026-10-30","duration":90,"time_tracking_category":{"id":30},
				"time_tracking_duration_type":null,"time_tracking_invoicing_type":{"id":11},"time_tracking_task":{"id":20},"wording":null}`,
		},
		{
			name:    "requires a company for company-scoped tools",
			tool:    "get_portfolio_company",
			args:    map[string]any{},
			wantErr: "company_id is required",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			api := &fakeExpertAPI{tasks: []json.RawMessage{json.RawMessage(task)}}
			session := connectExpert(t, NewExpertHandler(api, nil, 0, t.TempDir()))

			errText := callExpert(t, session, testCase.tool, testCase.args)
			if testCase.wantErr != "" {
				assert.Contains(t, errText, testCase.wantErr)
				return
			}
			require.Empty(t, errText)

			if testCase.wantQuery != nil {
				assert.Equal(t, *testCase.wantQuery, api.lastQuery)
			}
			if testCase.wantBody != "" {
				assert.JSONEq(t, testCase.wantBody, asJSON(t, api.lastBody))
			}
			assert.Equal(t, testCase.wantNexts, api.lastNexts)
		})
	}
}

func TestExpertAccountingAndFollowUpTools(
	t *testing.T,
) {
	testCases := []struct {
		name      string
		tool      string
		args      map[string]any
		wantErr   string
		wantQuery *tiime.ExpertQuery
		wantBody  string
	}{
		{
			name: "turns an accounting period into dates for the general ledger of an account",
			tool: "list_movements",
			args: map[string]any{"company_id": 7, "accounting_period_id": 3, "account_id": 12, "journal_codes": []string{"AC", "VT"}, "amount": "<0"},
			wantQuery: &tiime.ExpertQuery{Filters: map[string]string{
				"date": ">=2026-01-01,<=2026-12-31", "pcg_entry": "12", "journal_code": "AC,VT", "amount": "<0",
				"account_number": "", "account_number_prefixes": "", "wording": "", "piece_reference": "",
				"lettered": "", "validated": "", "has_attachment": "",
			}},
		},
		{
			name:    "rejects an unknown accounting period",
			tool:    "list_movements",
			args:    map[string]any{"company_id": 7, "accounting_period_id": 99},
			wantErr: "accounting period 99 not found",
		},
		{
			name:    "rejects an invalid amount condition",
			tool:    "list_movements",
			args:    map[string]any{"company_id": 7, "amount": "about 100"},
			wantErr: `invalid amount "about 100"`,
		},
		{
			name: "maps debit balances to negative amounts",
			tool: "get_trial_balance",
			args: map[string]any{"company_id": 7, "accounting_period_id": 3, "balance_side": "debit", "account_prefixes": []string{"411"}, "with_result": true},
			wantQuery: &tiime.ExpertQuery{Filters: map[string]string{
				"amount": "<0", "account_number_prefixes": "411", "account_numbers": "",
				"pcg_entry_has_movement": "", "pcg_entry_validated": "", "pcg_entry_balanced": "",
			}},
		},
		{
			name: "lists the VAT planning without ignored declarations by default",
			tool: "list_vat_planning",
			args: map[string]any{"deadline_to": "2026-11-30", "consistency_check_statuses": []string{"failed"}},
			wantQuery: &tiime.ExpertQuery{Filters: map[string]string{
				"vat_declaration_status":                   "draft|to_check|waiting_client|client_validation_requested|sent|accepted|declined",
				"vat_declaration_deadline_start_date":      "",
				"vat_declaration_deadline_end_date":        "2026-11-30",
				"vat_declaration_consistency_check_status": "failed",
				"vat_declaration_auto_process":             "", "out_of_deadline": "", "contributor_ids": "", "business_units": "",
			}},
		},
		{
			name: "sends the entity ID of a thread under its type",
			tool: "list_threads",
			args: map[string]any{"company_id": 7, "entity_type": "bank_transaction", "entity_id": 55, "status": "opened"},
			wantQuery: &tiime.ExpertQuery{Filters: map[string]string{
				"status": "opened", "types": "bank_transaction", "bank_transaction": "55", "category": "", "date": "",
			}},
		},
		{
			name:    "needs an entity type with an entity ID",
			tool:    "list_threads",
			args:    map[string]any{"company_id": 7, "entity_id": 55},
			wantErr: "entity_id needs entity_type",
		},
		{
			name:     "opens a thread about a bank transaction",
			tool:     "create_thread",
			args:     map[string]any{"company_id": 7, "entity_type": "bank_transaction", "entity_id": 55, "message": "Quel est ce paiement ?", "category": "customer"},
			wantBody: `{"category":"customer","entity":{"bank_transaction":{"id":55},"type":"bank_transaction"},"interlocutors":[],"messages":[{"content":"Quel est ce paiement ?"}],"tags":[]}`,
		},
		{
			name:    "refuses an empty message",
			tool:    "create_thread",
			args:    map[string]any{"company_id": 7, "entity_type": "task", "entity_id": 1, "message": "  "},
			wantErr: "message is required",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			api := &fakeExpertAPI{}
			session := connectExpert(t, NewExpertHandler(api, nil, 0, t.TempDir()))

			errText := callExpert(t, session, testCase.tool, testCase.args)
			if testCase.wantErr != "" {
				assert.Contains(t, errText, testCase.wantErr)
				return
			}
			require.Empty(t, errText)

			if testCase.wantQuery != nil {
				assert.Equal(t, *testCase.wantQuery, api.lastQuery)
			}
			if testCase.wantBody != "" {
				assert.JSONEq(t, testCase.wantBody, asJSON(t, api.lastBody))
			}
		})
	}
}

func TestRegisterExpertListsTools(
	t *testing.T,
) {
	session := connectExpert(t, NewExpertHandler(&fakeExpertAPI{}, nil, 0, t.TempDir()))

	result, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	assert.ElementsMatch(t, []string{
		"start_login", "complete_login",
		"get_expert_me", "list_portfolio_companies", "get_portfolio_company",
		"list_tasks", "create_task", "update_task", "delete_task", "set_task_billing_check",
		"get_time_tracking_options", "list_time_records", "get_time_summary", "save_time_record", "delete_time_record", "validate_time",
		"list_accounting_periods", "list_movements", "get_accounting_entry", "get_account_summary", "get_trial_balance",
		"list_chart_of_accounts", "list_journals", "list_company_labels", "validate_movements",
		"list_client_bank_transactions", "request_receipts", "cancel_receipt_requests",
		"list_vat_planning", "list_company_vat_declarations", "get_vat_declaration", "download_vat_declaration",
		"list_bilan_planning", "get_bilan", "list_client_indicators", "refresh_client_indicators", "list_items_to_relaunch",
		"list_threads", "create_thread", "post_thread_message", "update_thread", "list_notifications", "mark_notifications_read",
	}, names)
}
