package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// ExpertPortfolioAPI reads the client companies of the firm, its task
// plannings and its time tracking.
type ExpertPortfolioAPI interface {
	ExpertMe(
		ctx context.Context,
	) (json.RawMessage, error)
	ExpertCompanies(
		ctx context.Context,
		query tiime.ExpertQuery,
		window tiime.Range,
	) (tiime.RawPage, error)
	ExpertCompany(
		ctx context.Context,
		companyID int64,
		expand string,
	) (json.RawMessage, error)
	ExpertTasks(
		ctx context.Context,
		query tiime.ExpertQuery,
		window tiime.Range,
	) (tiime.RawPage, error)
	ExpertCompanyTasks(
		ctx context.Context,
		companyID int64,
	) ([]json.RawMessage, error)
	ExpertTaskTotals(
		ctx context.Context,
		query tiime.ExpertQuery,
	) (json.RawMessage, error)
	CreateExpertTask(
		ctx context.Context,
		task map[string]any,
	) (json.RawMessage, error)
	UpdateExpertTask(
		ctx context.Context,
		companyID int64,
		taskID int64,
		task map[string]any,
		applyNexts string,
	) (json.RawMessage, error)
	DeleteExpertTask(
		ctx context.Context,
		companyID int64,
		taskID int64,
		applyNexts string,
	) error
	SetExpertTaskBillingPointed(
		ctx context.Context,
		companyID int64,
		taskID int64,
		pointed bool,
	) (json.RawMessage, error)
	TimeTrackingReference(
		ctx context.Context,
		kind string,
		filters map[string]string,
	) (json.RawMessage, error)
	TimeRecords(
		ctx context.Context,
		userID int64,
		date string,
	) (json.RawMessage, error)
	TimeDays(
		ctx context.Context,
		ownerIDs string,
		startDate string,
		endDate string,
	) (json.RawMessage, error)
	CompanyTimeRecords(
		ctx context.Context,
		companyID int64,
		query tiime.ExpertQuery,
		window tiime.Range,
	) (tiime.RawPage, error)
	CompanyTimeTotal(
		ctx context.Context,
		companyID int64,
		query tiime.ExpertQuery,
	) (json.RawMessage, error)
	SaveTimeRecord(
		ctx context.Context,
		record map[string]any,
	) (json.RawMessage, error)
	DeleteTimeRecord(
		ctx context.Context,
		recordID int64,
	) error
	ValidateTimeRecords(
		ctx context.Context,
		userIDs []int64,
		startDate string,
		endDate string,
		validated bool,
	) error
}

var (
	taskStatuses           = []string{"draft", "waiting_client", "to_check", "to_correct", "to_be_returned", "to_supervise", "done", "ignored"}
	taskInvoicingStatuses  = []string{"to_bill", "to_bill_on_delivery", "billed", "na"}
	taskSortFields         = []string{"company", "task", "wording", "deadline_date", "responsable", "supervisor", "invoicing_status"}
	companyStatuses        = []string{"active", "mission_ended", "works_stopped", "in_creation_process", "initialization_in_progress"}
	timeRecordSortFields   = []string{"date", "time_tracking_task", "time_tracking_wording", "time_tracking_duration", "time_tracking_accounting_period", "time_tracking_invoicing_type", "time_tracking_owner"}
	timeTrackingReferences = []string{"categories", "tasks", "invoicing_types", "duration_types"}
	timeTaskUsages         = []string{"time_tracking", "prestation", "additional_mission"}
)

// ListPortfolioCompaniesInput filters the client companies of the firm.
type ListPortfolioCompaniesInput struct {
	PageInput
	Search         string  `json:"search,omitempty" jsonschema:"Free text search, e.g. part of the company name"`
	Status         string  `json:"status,omitempty" jsonschema:"active, mission_ended, works_stopped, in_creation_process or initialization_in_progress. Omit for all companies"`
	TaxRegimeCodes string  `json:"tax_regime_codes,omitempty" jsonschema:"Tax regime codes separated by |, e.g. ISRS|BNC|MICRO"`
	ClosingMonth   int     `json:"closing_month,omitempty" jsonschema:"Fiscal year closing month, 1 to 12"`
	ContributorIDs []int64 `json:"contributor_ids,omitempty" jsonschema:"Firm users in charge of the companies, see get_expert_me for your own ID"`
	BusinessUnits  []int64 `json:"business_unit_ids,omitempty" jsonschema:"Business units (offices) of the firm"`
	Sort           string  `json:"sort,omitempty" jsonschema:"name:asc or name:desc"`
}

// GetPortfolioCompanyInput selects a client company.
type GetPortfolioCompanyInput struct {
	ExpertCompanyInput
	Expand string `json:"expand,omitempty" jsonschema:"Optional blocks separated by commas: legal_company, pdp_config, thread"`
}

// ListTasksInput filters the task plannings (missions) of the firm.
type ListTasksInput struct {
	PageInput
	CompanyID      int64    `json:"company_id,omitempty" jsonschema:"Only the tasks of this client company. The other filters are then ignored and every task of the company is returned"`
	Search         string   `json:"search,omitempty" jsonschema:"Free text search"`
	Statuses       []string `json:"statuses,omitempty" jsonschema:"draft, waiting_client, to_check, to_correct, to_be_returned, to_supervise, done or ignored. Omit for every status except ignored"`
	DeadlineFrom   string   `json:"deadline_from,omitempty" jsonschema:"Deadline on or after this date, YYYY-MM-DD"`
	DeadlineTo     string   `json:"deadline_to,omitempty" jsonschema:"Deadline on or before this date, YYYY-MM-DD"`
	Overdue        *bool    `json:"overdue,omitempty" jsonschema:"true for tasks past their deadline only"`
	InvoicingState string   `json:"invoicing_status,omitempty" jsonschema:"to_bill, to_bill_on_delivery, billed or na"`
	BillingPointed *bool    `json:"billing_pointed,omitempty" jsonschema:"Filter on the billing check mark"`
	ResponsableIDs []int64  `json:"responsable_ids,omitempty" jsonschema:"Firm users responsible for the tasks"`
	SupervisorIDs  []int64  `json:"supervisor_ids,omitempty" jsonschema:"Firm users supervising the tasks"`
	TaskTypeIDs    []int64  `json:"task_type_ids,omitempty" jsonschema:"Task types, see get_time_tracking_options with kind tasks"`
	WithTotals     bool     `json:"with_totals,omitempty" jsonschema:"Also return the total amount of the matching tasks"`
	Sort           string   `json:"sort,omitempty" jsonschema:"<field>:<asc|desc> with field company, task, wording, deadline_date, responsable, supervisor or invoicing_status. Defaults to deadline_date:asc"`
}

// TasksOutput lists task plannings, with their totals when requested.
type TasksOutput struct {
	tiime.RawPage
	Totals map[string]any `json:"totals,omitempty"`
}

// RecurrenceInput repeats a task planning.
type RecurrenceInput struct {
	IntervalMonths   int    `json:"interval_months" jsonschema:"Months between two occurrences, at least 1"`
	EndOfMonth       bool   `json:"end_of_month,omitempty" jsonschema:"Place each occurrence at the end of the month instead of the day of the first deadline"`
	LastDeadlineDate string `json:"last_deadline_date,omitempty" jsonschema:"Date of the last occurrence, YYYY-MM-DD. Give it or occurrences"`
	Occurrences      int    `json:"occurrences,omitempty" jsonschema:"Number of occurrences. Give it or last_deadline_date"`
}

// CreateTaskInput describes a new task planning.
type CreateTaskInput struct {
	ExpertCompanyInput
	TaskTypeID      int64            `json:"task_type_id" jsonschema:"Task type, see get_time_tracking_options with kind tasks and usage additional_mission"`
	InvoicingTypeID int64            `json:"invoicing_type_id,omitempty" jsonschema:"Invoicing type, see get_time_tracking_options with kind invoicing_types. Defaults to the default type"`
	ResponsableID   int64            `json:"responsable_id,omitempty" jsonschema:"Firm user responsible for the task. Defaults to you"`
	SupervisorID    int64            `json:"supervisor_id,omitempty" jsonschema:"Firm user supervising the task"`
	DeadlineDate    string           `json:"deadline_date" jsonschema:"Deadline, YYYY-MM-DD"`
	Wording         string           `json:"wording,omitempty" jsonschema:"Label of the task"`
	InvoicingStatus string           `json:"invoicing_status,omitempty" jsonschema:"to_bill, to_bill_on_delivery, billed or na"`
	Amount          *float64         `json:"amount,omitempty" jsonschema:"Amount excluding VAT, required by some invoicing types"`
	Discount        *float64         `json:"discount,omitempty" jsonschema:"Discount, between 0 and amount"`
	Recurrence      *RecurrenceInput `json:"recurrence,omitempty" jsonschema:"Repeat the task"`
}

// UpdateTaskInput changes a task planning. Only the given fields change.
type UpdateTaskInput struct {
	ExpertCompanyInput
	TaskID          int64    `json:"task_id" jsonschema:"Task planning ID, see list_tasks"`
	Status          string   `json:"status,omitempty" jsonschema:"draft, waiting_client, to_check, to_correct, to_be_returned, to_supervise, done or ignored"`
	Wording         *string  `json:"wording,omitempty" jsonschema:"New label"`
	DeadlineDate    string   `json:"deadline_date,omitempty" jsonschema:"New deadline, YYYY-MM-DD"`
	InvoicingStatus string   `json:"invoicing_status,omitempty" jsonschema:"to_bill, to_bill_on_delivery, billed or na"`
	Amount          *float64 `json:"amount,omitempty" jsonschema:"New amount excluding VAT"`
	Discount        *float64 `json:"discount,omitempty" jsonschema:"New discount"`
	ResponsableID   int64    `json:"responsable_id,omitempty" jsonschema:"New responsible firm user"`
	SupervisorID    int64    `json:"supervisor_id,omitempty" jsonschema:"New supervising firm user"`
	ApplyToNext     bool     `json:"apply_to_next_occurrences,omitempty" jsonschema:"For a recurring task, also apply the change to the next occurrences"`
}

// TaskRefInput identifies a task planning.
type TaskRefInput struct {
	ExpertCompanyInput
	TaskID int64 `json:"task_id" jsonschema:"Task planning ID, see list_tasks"`
}

// DeleteTaskInput identifies the task planning to delete.
type DeleteTaskInput struct {
	TaskRefInput
	ApplyToNext bool `json:"delete_next_occurrences,omitempty" jsonschema:"For a recurring task, also delete the next occurrences"`
}

// BillingPointInput sets the billing check mark of a task planning.
type BillingPointInput struct {
	TaskRefInput
	Pointed bool `json:"pointed" jsonschema:"true to mark the task as checked for billing, false to remove the mark"`
}

// TimeTrackingOptionsInput selects time tracking reference data.
type TimeTrackingOptionsInput struct {
	Kind  string `json:"kind" jsonschema:"categories, tasks, invoicing_types or duration_types"`
	Usage string `json:"usage,omitempty" jsonschema:"For tasks: time_tracking (time entry), prestation (contract services) or additional_mission (task plannings)"`
}

// ListTimeRecordsInput selects time records.
type ListTimeRecordsInput struct {
	PageInput
	CompanyID       int64   `json:"company_id,omitempty" jsonschema:"Time recorded on this client company. Without it, the time of user_id on date is returned"`
	UserID          int64   `json:"user_id,omitempty" jsonschema:"Firm user. Defaults to you"`
	Date            string  `json:"date,omitempty" jsonschema:"Day of the records of a user, YYYY-MM-DD, defaults to today. Ignored with company_id"`
	From            string  `json:"from,omitempty" jsonschema:"With company_id: records on or after this date, YYYY-MM-DD"`
	To              string  `json:"to,omitempty" jsonschema:"With company_id: records on or before this date, YYYY-MM-DD"`
	TaskTypeID      int64   `json:"task_type_id,omitempty" jsonschema:"With company_id: only this task type"`
	InvoicingTypeID []int64 `json:"invoicing_type_ids,omitempty" jsonschema:"With company_id: only these invoicing types"`
	WithTotal       bool    `json:"with_total,omitempty" jsonschema:"With company_id: also return the total duration in minutes"`
	Sort            string  `json:"sort,omitempty" jsonschema:"With company_id: <field>:<asc|desc>, field one of date, time_tracking_task, time_tracking_wording, time_tracking_duration, time_tracking_accounting_period, time_tracking_invoicing_type, time_tracking_owner"`
}

// TimeRecordsOutput lists time records.
type TimeRecordsOutput struct {
	Items   []map[string]any `json:"items"`
	Offset  int              `json:"offset"`
	HasMore bool             `json:"has_more"`
	Total   map[string]any   `json:"total,omitempty"`
}

// TimeSummaryInput selects a period of time tracking.
type TimeSummaryInput struct {
	UserIDs   []int64 `json:"user_ids,omitempty" jsonschema:"Firm users. Defaults to you"`
	StartDate string  `json:"start_date" jsonschema:"First day, YYYY-MM-DD"`
	EndDate   string  `json:"end_date" jsonschema:"Last day, YYYY-MM-DD"`
}

// SaveTimeRecordInput creates or updates a time record.
type SaveTimeRecordInput struct {
	RecordID         int64  `json:"record_id,omitempty" jsonschema:"ID of the record to update. Omit to create a record"`
	Date             string `json:"date" jsonschema:"Day of the work, YYYY-MM-DD"`
	TaskTypeID       int64  `json:"task_type_id" jsonschema:"Task type, see get_time_tracking_options with kind tasks and usage time_tracking"`
	CategoryID       int64  `json:"category_id,omitempty" jsonschema:"Category. Defaults to the category of the task type"`
	CompanyID        int64  `json:"company_id,omitempty" jsonschema:"Client company, required by some categories"`
	DurationMinutes  int    `json:"duration_minutes" jsonschema:"Duration in minutes"`
	InvoicingTypeID  int64  `json:"invoicing_type_id,omitempty" jsonschema:"Invoicing type. Defaults to the default type"`
	AccountingPeriod int64  `json:"accounting_period_id,omitempty" jsonschema:"Accounting period (fiscal year) of the company. Do not combine with duration_type_id"`
	DurationTypeID   int64  `json:"duration_type_id,omitempty" jsonschema:"Duration type, see get_time_tracking_options. Do not combine with accounting_period_id"`
	Wording          string `json:"wording,omitempty" jsonschema:"Description of the work"`
}

// ValidateTimeInput validates the time of users over a period.
type ValidateTimeInput struct {
	TimeSummaryInput
	Validated bool `json:"validated" jsonschema:"true to validate the days, false to reopen them"`
}

// DeleteTimeRecordInput identifies a time record.
type DeleteTimeRecordInput struct {
	RecordID int64 `json:"record_id" jsonschema:"Time record ID"`
}

// DoneOutput reports a write without a returned resource.
type DoneOutput struct {
	Done bool `json:"done"`
}

func registerExpertPortfolio(
	server *mcp.Server,
	h *ExpertHandler,
) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := true

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_expert_me",
		Description: "Get the accounting firm user logged in to Tiime Expert: ID, name, business units. Use the ID for the user filters of the other tools.",
		Annotations: readOnly,
	}, h.GetMe)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_portfolio_companies",
		Description: "List the client companies of the accounting firm, with search and filters on mission status, tax regime, closing month, contributors and business units. Start here to find a company_id.",
		Annotations: readOnly,
	}, h.ListPortfolioCompanies)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_portfolio_company",
		Description: "Get the file of a client company: identity, tax and VAT regimes, mission dates, contributors, accounting configuration.",
		Annotations: readOnly,
	}, h.GetPortfolioCompany)

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_tasks",
		Description: "List the task plannings (missions with a deadline: bookkeeping, VAT, annual accounts, extra missions…) of the firm, " +
			"with filters on status, deadline, overdue, invoicing and people, or every task of one client company.",
		Annotations: readOnly,
	}, h.ListTasks)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_task",
		Description: "Create a task planning for a client company, optionally recurring every N months.",
	}, h.CreateTask)

	mcp.AddTool(server, &mcp.Tool{
		Name: "update_task",
		Description: "Change a task planning: status (e.g. done or waiting_client), label, deadline, invoicing, people. " +
			"Only the given fields change. Setting done on a to_bill_on_delivery task locks it and sends it to billing.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.UpdateTask)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_task",
		Description: "Delete a task planning, and optionally its next occurrences.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.DeleteTask)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_task_billing_check",
		Description: "Mark a task planning as checked for billing, or remove the mark.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.SetTaskBillingCheck)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_time_tracking_options",
		Description: "List time tracking reference data: categories, task types, invoicing types or duration types, with their IDs.",
		Annotations: readOnly,
	}, h.GetTimeTrackingOptions)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_time_records",
		Description: "List time records: the time spent on a client company over a period, or the records of a firm user for a day.",
		Annotations: readOnly,
	}, h.ListTimeRecords)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_time_summary",
		Description: "Get the recorded duration (minutes) and validation state of each day of a period, for firm users.",
		Annotations: readOnly,
	}, h.GetTimeSummary)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "save_time_record",
		Description: "Record time spent, or update a time record. Days already validated must be reopened first with validate_time.",
	}, h.SaveTimeRecord)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_time_record",
		Description: "Delete a time record.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.DeleteTimeRecord)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "validate_time",
		Description: "Validate the time records of firm users over a period, or reopen validated days.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.ValidateTime)
}

// GetMe implements the get_expert_me tool.
func (h *ExpertHandler) GetMe(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ struct{},
) (*mcp.CallToolResult, any, error) {
	me, err := h.api.ExpertMe(ctx)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(me), nil, nil
}

// ListPortfolioCompanies implements the list_portfolio_companies tool.
func (h *ExpertHandler) ListPortfolioCompanies(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListPortfolioCompaniesInput,
) (*mcp.CallToolResult, tiime.RawPage, error) {
	if err := checkEnum("status", in.Status, companyStatuses); err != nil {
		return nil, tiime.RawPage{}, err
	}
	if in.ClosingMonth < 0 || in.ClosingMonth > 12 {
		return nil, tiime.RawPage{}, fmt.Errorf("invalid closing_month %d, expected 1 to 12", in.ClosingMonth)
	}
	sort, err := sortParam(in.Sort, []string{"name"})
	if err != nil {
		return nil, tiime.RawPage{}, err
	}

	filters := companyStatusFilters(in.Status)
	filters["tax_regime_code"] = in.TaxRegimeCodes
	filters["contributor_ids"] = joinIDs(in.ContributorIDs, "|")
	// The expert application joins business units with commas, not pipes.
	filters["business_units"] = joinIDs(in.BusinessUnits, ",")
	if in.ClosingMonth != 0 {
		filters["closing_month"] = strconv.Itoa(in.ClosingMonth)
	}

	page, err := h.api.ExpertCompanies(ctx, tiime.ExpertQuery{Search: in.Search, Sort: sort, Filters: filters}, in.window())

	return nil, page, err
}

// companyStatusFilters maps a mission status to the flags of the expert
// application.
func companyStatusFilters(
	status string,
) map[string]string {
	switch status {
	case "active":
		return map[string]string{
			"mission_ended":              "false",
			"in_creation_process":        "false",
			"initialization_in_progress": "false",
			"mission_works_stopped":      "false",
		}
	case "mission_ended":
		return map[string]string{"mission_ended": "true"}
	case "works_stopped":
		return map[string]string{"mission_works_stopped": "true", "mission_ended": "false"}
	case "in_creation_process":
		return map[string]string{"in_creation_process": "true"}
	case "initialization_in_progress":
		return map[string]string{"initialization_in_progress": "true"}
	default:
		return map[string]string{}
	}
}

// GetPortfolioCompany implements the get_portfolio_company tool.
func (h *ExpertHandler) GetPortfolioCompany(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetPortfolioCompanyInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	company, err := h.api.ExpertCompany(ctx, companyID, in.Expand)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(company), nil, nil
}

// ListTasks implements the list_tasks tool.
func (h *ExpertHandler) ListTasks(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListTasksInput,
) (*mcp.CallToolResult, TasksOutput, error) {
	if in.CompanyID != 0 {
		tasks, err := h.api.ExpertCompanyTasks(ctx, in.CompanyID)
		if err != nil {
			return nil, TasksOutput{}, err
		}
		items, err := decodeObjects(tasks)
		if err != nil {
			return nil, TasksOutput{}, err
		}
		return nil, TasksOutput{RawPage: tiime.RawPage{Items: items}}, nil
	}

	query, err := in.query()
	if err != nil {
		return nil, TasksOutput{}, err
	}

	page, err := h.api.ExpertTasks(ctx, query, in.window())
	if err != nil {
		return nil, TasksOutput{}, err
	}
	out := TasksOutput{RawPage: page}

	if in.WithTotals {
		totals, err := h.api.ExpertTaskTotals(ctx, query)
		if err != nil {
			return nil, TasksOutput{}, err
		}
		if err := json.Unmarshal(totals, &out.Totals); err != nil {
			return nil, TasksOutput{}, fmt.Errorf(
				"decoding task totals: %w",
				err,
			)
		}
	}

	return nil, out, nil
}

func (in ListTasksInput) query() (tiime.ExpertQuery, error) {
	for _, status := range in.Statuses {
		if err := checkEnum("status", status, taskStatuses); err != nil {
			return tiime.ExpertQuery{}, err
		}
	}
	if err := checkEnum("invoicing_status", in.InvoicingState, taskInvoicingStatuses); err != nil {
		return tiime.ExpertQuery{}, err
	}
	deadline, err := dateRange(in.DeadlineFrom, in.DeadlineTo)
	if err != nil {
		return tiime.ExpertQuery{}, err
	}
	sort := in.Sort
	if sort == "" {
		sort = "deadline_date:asc"
	}
	if sort, err = sortParam(sort, taskSortFields); err != nil {
		return tiime.ExpertQuery{}, err
	}

	statuses := in.Statuses
	if len(statuses) == 0 {
		// Same default as the expert application: everything but ignored.
		statuses = taskStatuses[:len(taskStatuses)-1]
	}

	return tiime.ExpertQuery{
		Search: in.Search,
		Sort:   sort,
		Filters: map[string]string{
			"status":           strings.Join(statuses, "|"),
			"date":             deadline,
			"out_of_deadline":  boolParam(in.Overdue),
			"invoicing_status": in.InvoicingState,
			"billing_pointed":  boolParam(in.BillingPointed),
			"responsable_ids":  joinIDs(in.ResponsableIDs, "|"),
			"supervisor_ids":   joinIDs(in.SupervisorIDs, "|"),
			"task_ids":         joinIDs(in.TaskTypeIDs, "|"),
		},
	}, nil
}

// CreateTask implements the create_task tool.
func (h *ExpertHandler) CreateTask(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CreateTaskInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if err := in.validate(); err != nil {
		return nil, nil, err
	}

	responsableID := in.ResponsableID
	if responsableID == 0 {
		if responsableID, err = h.myID(ctx); err != nil {
			return nil, nil, err
		}
	}
	invoicingTypeID := in.InvoicingTypeID
	if invoicingTypeID == 0 {
		if invoicingTypeID, err = h.defaultInvoicingType(ctx); err != nil {
			return nil, nil, err
		}
	}

	task := map[string]any{
		"wording":                      in.Wording,
		"deadline_date":                in.DeadlineDate,
		"company":                      idRef(companyID),
		"time_tracking_task":           idRef(in.TaskTypeID),
		"time_tracking_invoicing_type": idRef(invoicingTypeID),
		"responsable":                  idRef(responsableID),
		"supervisor":                   idRef(in.SupervisorID),
		"invoicing_status":             nullable(in.InvoicingStatus),
		"amount":                       in.Amount,
		"discount":                     in.Discount,
		"task_scheduler":               nil,
	}
	if in.Recurrence != nil {
		task["task_scheduler"] = taskScheduler(task, in.Recurrence)
	}

	created, err := h.api.CreateExpertTask(ctx, task)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(created), nil, nil
}

func (in CreateTaskInput) validate() error {
	if in.TaskTypeID == 0 {
		return errors.New("task_type_id is required: see get_time_tracking_options with kind tasks")
	}
	if in.DeadlineDate == "" {
		return errors.New("deadline_date is required")
	}
	if err := checkDate(in.DeadlineDate); err != nil {
		return err
	}
	if err := checkEnum("invoicing_status", in.InvoicingStatus, taskInvoicingStatuses); err != nil {
		return err
	}
	if err := checkAmounts(in.Amount, in.Discount); err != nil {
		return err
	}
	if in.Recurrence == nil {
		return nil
	}

	r := in.Recurrence
	if r.IntervalMonths < 1 {
		return errors.New("recurrence.interval_months must be at least 1")
	}
	if (r.LastDeadlineDate == "") == (r.Occurrences == 0) {
		return errors.New("recurrence needs either last_deadline_date or occurrences")
	}
	if r.Occurrences < 0 {
		return errors.New("recurrence.occurrences must be positive")
	}

	return checkDate(r.LastDeadlineDate)
}

func checkAmounts(
	amount *float64,
	discount *float64,
) error {
	if amount != nil && *amount < 0 {
		return errors.New("amount must not be negative")
	}
	if discount == nil {
		return nil
	}
	if *discount < 0 {
		return errors.New("discount must not be negative")
	}
	if amount != nil && *discount > *amount {
		return errors.New("discount must not exceed amount")
	}

	return nil
}

// taskScheduler builds the recurrence of a task, which mirrors the task.
func taskScheduler(
	task map[string]any,
	r *RecurrenceInput,
) map[string]any {
	scheduler := map[string]any{
		"interval":            r.IntervalMonths,
		"day":                 nil,
		"first_deadline_date": task["deadline_date"],
		"last_deadline_date":  nullable(r.LastDeadlineDate),
		"occurence":           nil,
	}
	if r.Occurrences != 0 {
		scheduler["occurence"] = r.Occurrences
	}
	// A null day places each occurrence at the end of the month.
	if deadline, err := time.Parse(time.DateOnly, task["deadline_date"].(string)); err == nil && !r.EndOfMonth {
		scheduler["day"] = deadline.Day()
	}
	for _, key := range []string{"wording", "company", "responsable", "supervisor", "time_tracking_task", "time_tracking_invoicing_type", "invoicing_status", "amount", "discount"} {
		scheduler[key] = task[key]
	}

	return scheduler
}

// UpdateTask implements the update_task tool. The expert application always
// sends the whole task: the current task is read, changed, then sent back.
func (h *ExpertHandler) UpdateTask(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in UpdateTaskInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if err := in.validate(); err != nil {
		return nil, nil, err
	}

	current, err := h.findTask(ctx, companyID, in.TaskID)
	if err != nil {
		return nil, nil, err
	}

	task := taskBody(current)
	in.apply(task)
	if err := checkAmounts(floatField(task, "amount"), floatField(task, "discount")); err != nil {
		return nil, nil, err
	}

	applyNexts := ""
	if in.ApplyToNext {
		applyNexts, _ = task["deadline_date"].(string)
	}

	updated, err := h.api.UpdateExpertTask(ctx, companyID, in.TaskID, task, applyNexts)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(updated), nil, nil
}

func (in UpdateTaskInput) validate() error {
	if in.TaskID == 0 {
		return errors.New("task_id is required: see list_tasks")
	}
	if err := checkEnum("status", in.Status, taskStatuses); err != nil {
		return err
	}
	if err := checkEnum("invoicing_status", in.InvoicingStatus, taskInvoicingStatuses); err != nil {
		return err
	}

	return checkDate(in.DeadlineDate)
}

func (in UpdateTaskInput) apply(
	task map[string]any,
) {
	if in.Status != "" {
		task["status"] = in.Status
	}
	if in.Wording != nil {
		task["wording"] = *in.Wording
	}
	if in.DeadlineDate != "" {
		task["deadline_date"] = in.DeadlineDate
	}
	if in.InvoicingStatus != "" {
		task["invoicing_status"] = in.InvoicingStatus
	}
	if in.Amount != nil {
		task["amount"] = *in.Amount
	}
	if in.Discount != nil {
		task["discount"] = *in.Discount
	}
	if in.ResponsableID != 0 {
		task["responsable"] = idRef(in.ResponsableID)
	}
	if in.SupervisorID != 0 {
		task["supervisor"] = idRef(in.SupervisorID)
	}
}

// taskBody keeps the fields of a task that the expert application sends back.
func taskBody(
	task map[string]any,
) map[string]any {
	body := map[string]any{}
	for _, key := range []string{"id", "wording", "deadline_date", "status", "company", "task_scheduler", "responsable", "supervisor", "invoicing_status", "amount", "discount"} {
		body[key] = task[key]
	}
	for _, key := range []string{"time_tracking_task", "time_tracking_invoicing_type"} {
		body[key] = refOf(task[key])
	}

	return body
}

// refOf reduces a nested object to its {"id"} reference.
func refOf(
	value any,
) any {
	object, ok := value.(map[string]any)
	if !ok || object["id"] == nil {
		return nil
	}

	return map[string]any{"id": object["id"]}
}

func floatField(
	object map[string]any,
	key string,
) *float64 {
	value, ok := object[key].(float64)
	if !ok {
		return nil
	}

	return &value
}

func (h *ExpertHandler) findTask(
	ctx context.Context,
	companyID int64,
	taskID int64,
) (map[string]any, error) {
	tasks, err := h.api.ExpertCompanyTasks(ctx, companyID)
	if err != nil {
		return nil, err
	}

	items, err := decodeObjects(tasks)
	if err != nil {
		return nil, err
	}
	for _, task := range items {
		if id, ok := task["id"].(float64); ok && int64(id) == taskID {
			return task, nil
		}
	}

	return nil, fmt.Errorf("task %d not found in company %d: see list_tasks", taskID, companyID)
}

// DeleteTask implements the delete_task tool.
func (h *ExpertHandler) DeleteTask(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in DeleteTaskInput,
) (*mcp.CallToolResult, DoneOutput, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, DoneOutput{}, err
	}

	applyNexts := ""
	if in.ApplyToNext {
		task, err := h.findTask(ctx, companyID, in.TaskID)
		if err != nil {
			return nil, DoneOutput{}, err
		}
		applyNexts, _ = task["deadline_date"].(string)
	}

	if err := h.api.DeleteExpertTask(ctx, companyID, in.TaskID, applyNexts); err != nil {
		return nil, DoneOutput{}, err
	}

	return nil, DoneOutput{Done: true}, nil
}

// SetTaskBillingCheck implements the set_task_billing_check tool.
func (h *ExpertHandler) SetTaskBillingCheck(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in BillingPointInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	task, err := h.api.SetExpertTaskBillingPointed(ctx, companyID, in.TaskID, in.Pointed)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(task), nil, nil
}

// GetTimeTrackingOptions implements the get_time_tracking_options tool.
func (h *ExpertHandler) GetTimeTrackingOptions(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in TimeTrackingOptionsInput,
) (*mcp.CallToolResult, any, error) {
	if err := checkEnum("kind", in.Kind, timeTrackingReferences); err != nil || in.Kind == "" {
		return nil, nil, fmt.Errorf("kind is required, one of: categories, tasks, invoicing_types, duration_types")
	}
	if err := checkEnum("usage", in.Usage, timeTaskUsages); err != nil {
		return nil, nil, err
	}

	var filters map[string]string
	if in.Kind == "tasks" && in.Usage != "" {
		filters = map[string]string{in.Usage: "true"}
	}

	options, err := h.api.TimeTrackingReference(ctx, in.Kind, filters)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(options), nil, nil
}

// ListTimeRecords implements the list_time_records tool.
func (h *ExpertHandler) ListTimeRecords(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListTimeRecordsInput,
) (*mcp.CallToolResult, TimeRecordsOutput, error) {
	if in.CompanyID == 0 {
		return h.userTimeRecords(ctx, in)
	}

	period, err := dateRange(in.From, in.To)
	if err != nil {
		return nil, TimeRecordsOutput{}, err
	}
	sort, err := sortParam(in.Sort, timeRecordSortFields)
	if err != nil {
		return nil, TimeRecordsOutput{}, err
	}
	filters := map[string]string{
		"date":                         period,
		"time_tracking_invoicing_type": joinIDs(in.InvoicingTypeID, "|"),
	}
	if in.UserID != 0 {
		filters["user_id"] = strconv.FormatInt(in.UserID, 10)
	}
	if in.TaskTypeID != 0 {
		filters["gi_task_id"] = strconv.FormatInt(in.TaskTypeID, 10)
	}
	query := tiime.ExpertQuery{Sort: sort, Filters: filters}

	page, err := h.api.CompanyTimeRecords(ctx, in.CompanyID, query, in.window())
	if err != nil {
		return nil, TimeRecordsOutput{}, err
	}
	out := TimeRecordsOutput{Items: page.Items, Offset: page.Offset, HasMore: page.HasMore}

	if in.WithTotal {
		total, err := h.api.CompanyTimeTotal(ctx, in.CompanyID, query)
		if err != nil {
			return nil, TimeRecordsOutput{}, err
		}
		if err := json.Unmarshal(total, &out.Total); err != nil {
			return nil, TimeRecordsOutput{}, fmt.Errorf(
				"decoding time total: %w",
				err,
			)
		}
	}

	return nil, out, nil
}

func (h *ExpertHandler) userTimeRecords(
	ctx context.Context,
	in ListTimeRecordsInput,
) (*mcp.CallToolResult, TimeRecordsOutput, error) {
	if err := checkDate(in.Date); err != nil {
		return nil, TimeRecordsOutput{}, err
	}
	date := in.Date
	if date == "" {
		date = time.Now().Format(time.DateOnly)
	}

	userID := in.UserID
	if userID == 0 {
		var err error
		if userID, err = h.myID(ctx); err != nil {
			return nil, TimeRecordsOutput{}, err
		}
	}

	raw, err := h.api.TimeRecords(ctx, userID, date)
	if err != nil {
		return nil, TimeRecordsOutput{}, err
	}

	var items []map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, TimeRecordsOutput{}, fmt.Errorf(
				"decoding time records: %w",
				err,
			)
		}
	}
	if items == nil {
		items = []map[string]any{}
	}

	return nil, TimeRecordsOutput{Items: items}, nil
}

// GetTimeSummary implements the get_time_summary tool.
func (h *ExpertHandler) GetTimeSummary(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in TimeSummaryInput,
) (*mcp.CallToolResult, any, error) {
	userIDs, err := h.timePeriod(ctx, in)
	if err != nil {
		return nil, nil, err
	}

	days, err := h.api.TimeDays(ctx, joinIDs(userIDs, "|"), in.StartDate, in.EndDate)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(days), nil, nil
}

// timePeriod checks a period and defaults the users to the logged in one.
func (h *ExpertHandler) timePeriod(
	ctx context.Context,
	in TimeSummaryInput,
) ([]int64, error) {
	if in.StartDate == "" || in.EndDate == "" {
		return nil, errors.New("start_date and end_date are required")
	}
	if _, err := dateRange(in.StartDate, in.EndDate); err != nil {
		return nil, err
	}
	if in.StartDate > in.EndDate {
		return nil, errors.New("start_date must not be after end_date")
	}

	if len(in.UserIDs) > 0 {
		return in.UserIDs, nil
	}

	me, err := h.myID(ctx)
	if err != nil {
		return nil, err
	}

	return []int64{me}, nil
}

// SaveTimeRecord implements the save_time_record tool.
func (h *ExpertHandler) SaveTimeRecord(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in SaveTimeRecordInput,
) (*mcp.CallToolResult, any, error) {
	if err := in.validate(); err != nil {
		return nil, nil, err
	}

	categoryID := in.CategoryID
	if categoryID == 0 {
		var err error
		if categoryID, err = h.taskCategory(ctx, in.TaskTypeID); err != nil {
			return nil, nil, err
		}
	}
	invoicingTypeID := in.InvoicingTypeID
	if invoicingTypeID == 0 {
		var err error
		if invoicingTypeID, err = h.defaultInvoicingType(ctx); err != nil {
			return nil, nil, err
		}
	}

	record := map[string]any{
		"date":                         in.Date,
		"company":                      idRef(in.CompanyID),
		"accounting_period":            idRef(in.AccountingPeriod),
		"wording":                      nullable(in.Wording),
		"duration":                     in.DurationMinutes,
		"time_tracking_task":           idRef(in.TaskTypeID),
		"time_tracking_category":       idRef(categoryID),
		"time_tracking_invoicing_type": idRef(invoicingTypeID),
		"time_tracking_duration_type":  idRef(in.DurationTypeID),
	}
	if in.RecordID != 0 {
		record["id"] = in.RecordID
	}

	saved, err := h.api.SaveTimeRecord(ctx, record)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(saved), nil, nil
}

func (in SaveTimeRecordInput) validate() error {
	if in.Date == "" {
		return errors.New("date is required")
	}
	if err := checkDate(in.Date); err != nil {
		return err
	}
	if in.TaskTypeID == 0 {
		return errors.New("task_type_id is required: see get_time_tracking_options with kind tasks")
	}
	if in.DurationMinutes <= 0 {
		return errors.New("duration_minutes must be positive")
	}
	if in.AccountingPeriod != 0 && in.DurationTypeID != 0 {
		return errors.New("give accounting_period_id or duration_type_id, not both")
	}

	return nil
}

// DeleteTimeRecord implements the delete_time_record tool.
func (h *ExpertHandler) DeleteTimeRecord(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in DeleteTimeRecordInput,
) (*mcp.CallToolResult, DoneOutput, error) {
	if in.RecordID == 0 {
		return nil, DoneOutput{}, errors.New("record_id is required: see list_time_records")
	}
	if err := h.api.DeleteTimeRecord(ctx, in.RecordID); err != nil {
		return nil, DoneOutput{}, err
	}

	return nil, DoneOutput{Done: true}, nil
}

// ValidateTime implements the validate_time tool.
func (h *ExpertHandler) ValidateTime(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ValidateTimeInput,
) (*mcp.CallToolResult, DoneOutput, error) {
	userIDs, err := h.timePeriod(ctx, in.TimeSummaryInput)
	if err != nil {
		return nil, DoneOutput{}, err
	}

	if err := h.api.ValidateTimeRecords(ctx, userIDs, in.StartDate, in.EndDate, in.Validated); err != nil {
		return nil, DoneOutput{}, err
	}

	return nil, DoneOutput{Done: true}, nil
}

// myID returns the ID of the firm user logged in.
func (h *ExpertHandler) myID(
	ctx context.Context,
) (int64, error) {
	raw, err := h.api.ExpertMe(ctx)
	if err != nil {
		return 0, err
	}

	var me struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &me); err != nil || me.ID == 0 {
		return 0, errors.New("cannot read the ID of the logged in user: pass the user ID explicitly")
	}

	return me.ID, nil
}

// defaultInvoicingType returns the invoicing type marked as default.
func (h *ExpertHandler) defaultInvoicingType(
	ctx context.Context,
) (int64, error) {
	raw, err := h.api.TimeTrackingReference(ctx, "invoicing_types", nil)
	if err != nil {
		return 0, err
	}

	var types []struct {
		ID      int64 `json:"id"`
		Default bool  `json:"default"`
	}
	if err := json.Unmarshal(raw, &types); err != nil {
		return 0, fmt.Errorf(
			"decoding invoicing types: %w",
			err,
		)
	}
	for _, invoicingType := range types {
		if invoicingType.Default {
			return invoicingType.ID, nil
		}
	}

	return 0, errors.New("no default invoicing type: pass invoicing_type_id, see get_time_tracking_options")
}

// taskCategory returns the category of a time tracking task type.
func (h *ExpertHandler) taskCategory(
	ctx context.Context,
	taskTypeID int64,
) (int64, error) {
	raw, err := h.api.TimeTrackingReference(ctx, "tasks", nil)
	if err != nil {
		return 0, err
	}

	var tasks []struct {
		ID       int64 `json:"id"`
		Category struct {
			ID int64 `json:"id"`
		} `json:"time_tracking_category"`
	}
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return 0, fmt.Errorf(
			"decoding time tracking tasks: %w",
			err,
		)
	}
	for _, task := range tasks {
		if task.ID == taskTypeID && task.Category.ID != 0 {
			return task.Category.ID, nil
		}
	}

	return 0, fmt.Errorf("task type %d not found: pass category_id, see get_time_tracking_options", taskTypeID)
}

// decodeObjects decodes a list of JSON objects.
func decodeObjects(
	raws []json.RawMessage,
) ([]map[string]any, error) {
	items := make([]map[string]any, 0, len(raws))
	for _, raw := range raws {
		var item map[string]any
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf(
				"decoding Tiime response: %w",
				err,
			)
		}
		items = append(items, item)
	}

	return items, nil
}

// nullable sends an empty string as null, as the expert application does.
func nullable(
	value string,
) any {
	if value == "" {
		return nil
	}

	return value
}
