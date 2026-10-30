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

// ExpertFollowUpAPI follows the portfolio: VAT and annual accounts plannings,
// client indicators, messaging and notifications.
type ExpertFollowUpAPI interface {
	VATPlanning(
		ctx context.Context,
		query tiime.ExpertQuery,
		window tiime.Range,
	) (tiime.RawPage, error)
	CompanyVATDeclarations(
		ctx context.Context,
		companyID int64,
		endDate string,
	) (json.RawMessage, error)
	VATDeclaration(
		ctx context.Context,
		companyID int64,
		declarationID int64,
	) (json.RawMessage, error)
	VATConsistencyChecks(
		ctx context.Context,
		companyID int64,
		declarationID int64,
	) (json.RawMessage, error)
	VATDeclarationFile(
		ctx context.Context,
		companyID int64,
		declarationID int64,
		docType string,
	) ([]byte, string, error)
	BilanPlanning(
		ctx context.Context,
		query tiime.ExpertQuery,
		window tiime.Range,
	) (tiime.RawPage, error)
	Bilan(
		ctx context.Context,
		companyID int64,
		bilanID int64,
	) (json.RawMessage, error)
	ClientIndicators(
		ctx context.Context,
		query tiime.ExpertQuery,
		window tiime.Range,
	) (tiime.RawPage, error)
	RefreshClientIndicators(
		ctx context.Context,
		companyID int64,
	) (json.RawMessage, error)
	ItemsToRelaunch(
		ctx context.Context,
		indicatorID int64,
		window tiime.Range,
	) (tiime.RawPage, error)
	ExpertThreads(
		ctx context.Context,
		companyID int64,
		query tiime.ExpertQuery,
		window tiime.Range,
	) (tiime.RawPage, error)
	CreateExpertThread(
		ctx context.Context,
		companyID int64,
		thread map[string]any,
	) (json.RawMessage, error)
	PostExpertThreadMessage(
		ctx context.Context,
		companyID int64,
		threadID int64,
		content string,
	) (json.RawMessage, error)
	ExpertThreadAction(
		ctx context.Context,
		companyID int64,
		threadID int64,
		action string,
	) error
	Notifications(
		ctx context.Context,
		read string,
		window tiime.Range,
	) (tiime.RawPage, error)
	NotificationCount(
		ctx context.Context,
	) (json.RawMessage, error)
	MarkNotificationRead(
		ctx context.Context,
		notificationID int64,
	) error
	MarkAllNotificationsRead(
		ctx context.Context,
	) error
}

var (
	vatStatuses          = []string{"draft", "to_check", "waiting_client", "client_validation_requested", "sent", "accepted", "declined", "ignored"}
	vatConsistencyStates = []string{"completed", "warning", "failed"}
	vatSortFields        = []string{"company", "cerfa", "deadline_date", "status", "status_at"}
	vatDocumentTypes     = []string{"vat_declaration", "acknowledgment_receipt", "fiscal_declaration", "proofs_of_filing", "vat_deposit"}
	bilanStatuses        = []string{"initialized", "draft", "doc_requested", "to_check", "to_correct", "to_be_returned", "to_supervise", "waiting_client", "done", "ignored"}
	bilanSortFields      = []string{"company", "end_date", "responsable", "bilan_status", "bilan_status_at", "liasse_status", "liasse_status_at", "2572_status", "2573_status"}
	indicatorSortFields  = []string{"name", "customer_access_user_on_company", "customer_exchanges_not_processed", "customer_exchanges_waiting_response", "document_requests", "customer_items_to_complete", "bank_account_states_warning", "temporary_accounting_entries", "tiime_business_status"}
	threadStatuses       = []string{"opened", "closed", "archived"}
	threadCategories     = []string{"customer", "accountant"}
	threadEntityTypes    = []string{"accounting_period", "vat_declaration", "task", "bank_transaction", "2571"}
	threadActions        = []string{"read", "close", "unclose", "archive", "unarchive"}
)

// PortfolioFilterInput narrows a portfolio planning to some companies.
type PortfolioFilterInput struct {
	CompanyStatus  string  `json:"company_status,omitempty" jsonschema:"active, mission_ended, works_stopped, in_creation_process or initialization_in_progress"`
	ContributorIDs []int64 `json:"contributor_ids,omitempty" jsonschema:"Firm users in charge of the companies"`
	BusinessUnits  []int64 `json:"business_unit_ids,omitempty" jsonschema:"Business units (offices) of the firm"`
}

func (in PortfolioFilterInput) filters() (map[string]string, error) {
	if err := checkEnum("company_status", in.CompanyStatus, companyStatuses); err != nil {
		return nil, err
	}

	filters := companyStatusFilters(in.CompanyStatus)
	filters["contributor_ids"] = joinIDs(in.ContributorIDs, "|")
	filters["business_units"] = joinIDs(in.BusinessUnits, ",")

	return filters, nil
}

// VATPlanningInput filters the VAT declarations of the portfolio.
type VATPlanningInput struct {
	PageInput
	PortfolioFilterInput
	Search            string   `json:"search,omitempty" jsonschema:"Free text search, e.g. a company name"`
	Statuses          []string `json:"statuses,omitempty" jsonschema:"draft, to_check, waiting_client, client_validation_requested, sent, accepted, declined or ignored. Omit for every status except ignored"`
	DeadlineFrom      string   `json:"deadline_from,omitempty" jsonschema:"Deadline on or after this date, YYYY-MM-DD"`
	DeadlineTo        string   `json:"deadline_to,omitempty" jsonschema:"Deadline on or before this date, YYYY-MM-DD"`
	Overdue           *bool    `json:"overdue,omitempty" jsonschema:"true for late declarations only"`
	DueWithinDays     int      `json:"due_within_days,omitempty" jsonschema:"Deadline within this number of days"`
	ConsistencyChecks []string `json:"consistency_check_statuses,omitempty" jsonschema:"completed, warning or failed"`
	AutoProcess       *bool    `json:"auto_process,omitempty" jsonschema:"Filter on automatic processing"`
	Sort              string   `json:"sort,omitempty" jsonschema:"<field>:<asc|desc>, field one of company, cerfa, deadline_date, status, status_at"`
}

// VATDeclarationsInput selects the VAT declarations of a client company.
type VATDeclarationsInput struct {
	ExpertCompanyInput
	EndDate string `json:"end_date,omitempty" jsonschema:"Only the declaration whose period ends on this date, YYYY-MM-DD"`
}

// VATDeclarationInput identifies a VAT declaration.
type VATDeclarationInput struct {
	ExpertCompanyInput
	DeclarationID int64 `json:"declaration_id" jsonschema:"VAT declaration ID, see list_vat_planning or list_company_vat_declarations"`
}

// VATFileInput selects a document of a VAT declaration.
type VATFileInput struct {
	VATDeclarationInput
	Type string `json:"type,omitempty" jsonschema:"vat_declaration (default), acknowledgment_receipt, fiscal_declaration, proofs_of_filing or vat_deposit"`
}

// FileOutput reports where a file was saved.
type FileOutput struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

// BilanPlanningInput filters the annual accounts of the portfolio.
type BilanPlanningInput struct {
	PageInput
	PortfolioFilterInput
	Search         string   `json:"search,omitempty" jsonschema:"Free text search, e.g. a company name"`
	Statuses       []string `json:"statuses,omitempty" jsonschema:"initialized, draft, doc_requested, to_check, to_correct, to_be_returned, to_supervise, waiting_client, done or ignored. Omit for every status except ignored"`
	ClosingFrom    string   `json:"closing_from,omitempty" jsonschema:"Fiscal year closing on or after this date, YYYY-MM-DD"`
	ClosingTo      string   `json:"closing_to,omitempty" jsonschema:"Fiscal year closing on or before this date, YYYY-MM-DD"`
	ResponsableIDs []int64  `json:"responsable_ids,omitempty" jsonschema:"Firm users responsible for the annual accounts"`
	TaxRegimes     string   `json:"tax_regime_codes,omitempty" jsonschema:"Tax regime codes separated by |, e.g. ISRS|BNC"`
	Sort           string   `json:"sort,omitempty" jsonschema:"<field>:<asc|desc>, field one of company, end_date, responsable, bilan_status, bilan_status_at, liasse_status, liasse_status_at, 2572_status, 2573_status"`
}

// BilanInput identifies annual accounts.
type BilanInput struct {
	ExpertCompanyInput
	BilanID int64 `json:"bilan_id" jsonschema:"Annual accounts ID, see list_bilan_planning"`
}

// ClientIndicatorsInput filters the client indicators of the portfolio.
type ClientIndicatorsInput struct {
	PageInput
	PortfolioFilterInput
	Search string `json:"search,omitempty" jsonschema:"Free text search, e.g. a company name"`
	Valid  *bool  `json:"valid,omitempty" jsonschema:"false for the companies with something to follow up only"`
	Sort   string `json:"sort,omitempty" jsonschema:"<field>:<asc|desc>, field one of name, customer_items_to_complete, customer_exchanges_waiting_response, customer_exchanges_not_processed, document_requests, bank_account_states_warning, temporary_accounting_entries. Defaults to name:asc"`
}

// ItemsToRelaunchInput selects an indicator row.
type ItemsToRelaunchInput struct {
	PageInput
	IndicatorID int64 `json:"indicator_id" jsonschema:"Indicator row ID (id field of list_client_indicators, not the company ID)"`
}

// ListThreadsInput filters the threads with a client company.
type ListThreadsInput struct {
	ExpertCompanyInput
	PageInput
	Status     string `json:"status,omitempty" jsonschema:"opened, closed or archived. Omit for all"`
	Category   string `json:"category,omitempty" jsonschema:"accountant (waiting for the firm) or customer (waiting for the client)"`
	EntityType string `json:"entity_type,omitempty" jsonschema:"Threads about accounting_period, vat_declaration, task, bank_transaction or 2571 (corporate tax instalment)"`
	EntityID   int64  `json:"entity_id,omitempty" jsonschema:"With entity_type: the ID of that entity"`
	Search     string `json:"search,omitempty" jsonschema:"Free text search"`
	From       string `json:"from,omitempty" jsonschema:"Created on or after this date, YYYY-MM-DD"`
	To         string `json:"to,omitempty" jsonschema:"Created on or before this date, YYYY-MM-DD"`
}

// CreateThreadInput opens a thread with a client company.
type CreateThreadInput struct {
	ExpertCompanyInput
	EntityType string `json:"entity_type" jsonschema:"What the thread is about: accounting_period, vat_declaration, task, bank_transaction or 2571"`
	EntityID   int64  `json:"entity_id" jsonschema:"ID of that entity"`
	Message    string `json:"message" jsonschema:"First message, visible to the client"`
	Category   string `json:"category,omitempty" jsonschema:"customer when an answer is expected from the client, accountant otherwise"`
}

// ThreadMessageInput posts a message.
type ThreadMessageInput struct {
	ExpertCompanyInput
	ThreadID int64  `json:"thread_id" jsonschema:"Thread ID, see list_threads"`
	Content  string `json:"content" jsonschema:"Message, visible to the client"`
}

// ThreadActionInput changes the state of a thread.
type ThreadActionInput struct {
	ExpertCompanyInput
	ThreadID int64  `json:"thread_id" jsonschema:"Thread ID, see list_threads"`
	Action   string `json:"action" jsonschema:"read, close, unclose, archive or unarchive"`
}

// NotificationsInput filters the notifications of the firm user.
type NotificationsInput struct {
	PageInput
	UnreadOnly bool `json:"unread_only,omitempty" jsonschema:"Only the unread notifications"`
}

// NotificationsOutput lists notifications with the unread count.
type NotificationsOutput struct {
	tiime.RawPage
	Unread map[string]any `json:"unread"`
}

// MarkNotificationsInput marks notifications as read.
type MarkNotificationsInput struct {
	NotificationIDs []int64 `json:"notification_ids,omitempty" jsonschema:"Notifications to mark as read"`
	All             bool    `json:"all,omitempty" jsonschema:"Mark every notification as read"`
}

func registerExpertFollowUp(
	server *mcp.Server,
	h *ExpertHandler,
) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_vat_planning",
		Description: "List the VAT declarations to handle across the portfolio, by deadline, status, consistency checks or company. " +
			"Defaults to every status except ignored.",
		Annotations: readOnly,
	}, h.ListVATPlanning)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_company_vat_declarations",
		Description: "List the VAT declarations of a client company.",
		Annotations: readOnly,
	}, h.ListCompanyVATDeclarations)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_vat_declaration",
		Description: "Get a VAT declaration of a client company with its form content and its consistency checks.",
		Annotations: readOnly,
	}, h.GetVATDeclaration)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "download_vat_declaration",
		Description: "Download a document of a VAT declaration (the declaration itself, acknowledgment receipt, proofs of filing…) to the local download directory.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.DownloadVATDeclaration)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_bilan_planning",
		Description: "List the annual accounts (bilans) to produce across the portfolio, by closing date, status, responsible or company.",
		Annotations: readOnly,
	}, h.ListBilanPlanning)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_bilan",
		Description: "Get the annual accounts of a client company: status, tax returns, dates, accounting result and consistency checks.",
		Annotations: readOnly,
	}, h.GetBilan)

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_client_indicators",
		Description: "List, for each client company, what the client still has to do: items to complete (missing receipts), unanswered " +
			"messages, document requests, bank connections needing action, suspense account entries, app access. " +
			"Use valid=false to find the clients to follow up.",
		Annotations: readOnly,
	}, h.ListClientIndicators)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "refresh_client_indicators",
		Description: "Recompute the client indicators of a company, e.g. after requesting receipts.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.RefreshClientIndicators)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_items_to_relaunch",
		Description: "List the bank transactions of a client with a pending receipt request, from an indicator row of list_client_indicators.",
		Annotations: readOnly,
	}, h.ListItemsToRelaunch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_threads",
		Description: "List the discussion threads with a client company, with their messages, filterable by status, waiting side and subject.",
		Annotations: readOnly,
	}, h.ListThreads)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_thread",
		Description: "Open a discussion thread with a client company about a bank transaction, a task, a VAT declaration or a fiscal year. The client sees the message.",
	}, h.CreateThread)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_thread_message",
		Description: "Post a message in a discussion thread with a client company. The client sees the message.",
	}, h.PostThreadMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_thread",
		Description: "Mark a thread as read, close or reopen it, archive or unarchive it.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.UpdateThread)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_notifications",
		Description: "List the notifications of the firm user, with the number of unread ones.",
		Annotations: readOnly,
	}, h.ListNotifications)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mark_notifications_read",
		Description: "Mark notifications of the firm user as read, or all of them.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.MarkNotificationsRead)
}

// ListVATPlanning implements the list_vat_planning tool.
func (h *ExpertHandler) ListVATPlanning(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VATPlanningInput,
) (*mcp.CallToolResult, tiime.RawPage, error) {
	query, err := in.query()
	if err != nil {
		return nil, tiime.RawPage{}, err
	}

	page, err := h.api.VATPlanning(ctx, query, in.window())

	return nil, page, err
}

func (in VATPlanningInput) query() (tiime.ExpertQuery, error) {
	filters, err := in.filters()
	if err != nil {
		return tiime.ExpertQuery{}, err
	}
	if err := checkEnums("status", in.Statuses, vatStatuses); err != nil {
		return tiime.ExpertQuery{}, err
	}
	if err := checkEnums("consistency_check_status", in.ConsistencyChecks, vatConsistencyStates); err != nil {
		return tiime.ExpertQuery{}, err
	}
	for _, date := range []string{in.DeadlineFrom, in.DeadlineTo} {
		if err := checkDate(date); err != nil {
			return tiime.ExpertQuery{}, err
		}
	}
	if in.DueWithinDays < 0 {
		return tiime.ExpertQuery{}, errors.New("due_within_days must be positive")
	}
	sort, err := sortParam(in.Sort, vatSortFields)
	if err != nil {
		return tiime.ExpertQuery{}, err
	}

	statuses := in.Statuses
	if len(statuses) == 0 {
		statuses = vatStatuses[:len(vatStatuses)-1]
	}

	// The VAT planning takes plain dates, without the >= and <= prefixes.
	filters["vat_declaration_deadline_start_date"] = in.DeadlineFrom
	filters["vat_declaration_deadline_end_date"] = in.DeadlineTo
	filters["vat_declaration_status"] = strings.Join(statuses, "|")
	filters["vat_declaration_consistency_check_status"] = strings.Join(in.ConsistencyChecks, "|")
	filters["vat_declaration_auto_process"] = boolParam(in.AutoProcess)
	filters["out_of_deadline"] = boolParam(in.Overdue)
	if in.DueWithinDays > 0 {
		filters["emergency_days"] = strconv.Itoa(in.DueWithinDays)
	}

	return tiime.ExpertQuery{Search: in.Search, Sort: sort, Filters: filters}, nil
}

// ListCompanyVATDeclarations implements the list_company_vat_declarations tool.
func (h *ExpertHandler) ListCompanyVATDeclarations(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VATDeclarationsInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if err := checkDate(in.EndDate); err != nil {
		return nil, nil, err
	}

	declarations, err := h.api.CompanyVATDeclarations(ctx, companyID, in.EndDate)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(declarations), nil, nil
}

// GetVATDeclaration implements the get_vat_declaration tool.
func (h *ExpertHandler) GetVATDeclaration(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VATDeclarationInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if in.DeclarationID == 0 {
		return nil, nil, errors.New("declaration_id is required: see list_company_vat_declarations")
	}

	parts := map[string]json.RawMessage{}
	if parts["declaration"], err = h.api.VATDeclaration(ctx, companyID, in.DeclarationID); err != nil {
		return nil, nil, err
	}
	if parts["consistency_checks"], err = h.api.VATConsistencyChecks(ctx, companyID, in.DeclarationID); err != nil {
		return nil, nil, err
	}

	return jsonResult(parts)
}

// DownloadVATDeclaration implements the download_vat_declaration tool.
func (h *ExpertHandler) DownloadVATDeclaration(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VATFileInput,
) (*mcp.CallToolResult, FileOutput, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, FileOutput{}, err
	}
	if in.DeclarationID == 0 {
		return nil, FileOutput{}, errors.New("declaration_id is required: see list_company_vat_declarations")
	}
	docType := in.Type
	if docType == "" {
		docType = "vat_declaration"
	}
	if err := checkEnum("type", docType, vatDocumentTypes); err != nil {
		return nil, FileOutput{}, err
	}

	content, filename, err := h.api.VATDeclarationFile(ctx, companyID, in.DeclarationID, docType)
	if err != nil {
		return nil, FileOutput{}, err
	}

	name := fmt.Sprintf("tiime-vat-%d-%s.pdf", in.DeclarationID, docType)
	if filename != "" {
		name = safeName(filename)
	}
	path, err := saveFile(h.downloadDir, name, content)
	if err != nil {
		return nil, FileOutput{}, err
	}

	return nil, FileOutput{Path: path, Bytes: len(content)}, nil
}

// ListBilanPlanning implements the list_bilan_planning tool.
func (h *ExpertHandler) ListBilanPlanning(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in BilanPlanningInput,
) (*mcp.CallToolResult, tiime.RawPage, error) {
	filters, err := in.filters()
	if err != nil {
		return nil, tiime.RawPage{}, err
	}
	if err := checkEnums("status", in.Statuses, bilanStatuses); err != nil {
		return nil, tiime.RawPage{}, err
	}
	closing, err := dateRange(in.ClosingFrom, in.ClosingTo)
	if err != nil {
		return nil, tiime.RawPage{}, err
	}
	sort, err := sortParam(in.Sort, bilanSortFields)
	if err != nil {
		return nil, tiime.RawPage{}, err
	}

	statuses := in.Statuses
	if len(statuses) == 0 {
		statuses = bilanStatuses[:len(bilanStatuses)-1]
	}
	filters["bilan_status"] = strings.Join(statuses, "|")
	filters["accounting_period_end_date"] = closing
	filters["responsable_ids"] = joinIDs(in.ResponsableIDs, "|")
	filters["tax_regime_code"] = in.TaxRegimes

	page, err := h.api.BilanPlanning(ctx, tiime.ExpertQuery{Search: in.Search, Sort: sort, Filters: filters}, in.window())

	return nil, page, err
}

// GetBilan implements the get_bilan tool.
func (h *ExpertHandler) GetBilan(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in BilanInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if in.BilanID == 0 {
		return nil, nil, errors.New("bilan_id is required: see list_bilan_planning")
	}

	bilan, err := h.api.Bilan(ctx, companyID, in.BilanID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(bilan), nil, nil
}

// ListClientIndicators implements the list_client_indicators tool.
func (h *ExpertHandler) ListClientIndicators(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ClientIndicatorsInput,
) (*mcp.CallToolResult, tiime.RawPage, error) {
	filters, err := in.filters()
	if err != nil {
		return nil, tiime.RawPage{}, err
	}
	sort := in.Sort
	if sort == "" {
		sort = "name:asc"
	}
	if sort, err = sortParam(sort, indicatorSortFields); err != nil {
		return nil, tiime.RawPage{}, err
	}
	filters["valid"] = boolParam(in.Valid)

	page, err := h.api.ClientIndicators(ctx, tiime.ExpertQuery{Search: in.Search, Sort: sort, Filters: filters}, in.window())

	return nil, page, err
}

// RefreshClientIndicators implements the refresh_client_indicators tool.
func (h *ExpertHandler) RefreshClientIndicators(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ExpertCompanyInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	indicators, err := h.api.RefreshClientIndicators(ctx, companyID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(indicators), nil, nil
}

// ListItemsToRelaunch implements the list_items_to_relaunch tool.
func (h *ExpertHandler) ListItemsToRelaunch(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ItemsToRelaunchInput,
) (*mcp.CallToolResult, tiime.RawPage, error) {
	if in.IndicatorID == 0 {
		return nil, tiime.RawPage{}, errors.New("indicator_id is required: use the id of a row of list_client_indicators")
	}

	page, err := h.api.ItemsToRelaunch(ctx, in.IndicatorID, in.window())

	return nil, page, err
}

// ListThreads implements the list_threads tool.
func (h *ExpertHandler) ListThreads(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListThreadsInput,
) (*mcp.CallToolResult, tiime.RawPage, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, tiime.RawPage{}, err
	}
	if err := checkEnum("status", in.Status, threadStatuses); err != nil {
		return nil, tiime.RawPage{}, err
	}
	if err := checkEnum("category", in.Category, threadCategories); err != nil {
		return nil, tiime.RawPage{}, err
	}
	if err := checkEnum("entity_type", in.EntityType, threadEntityTypes); err != nil {
		return nil, tiime.RawPage{}, err
	}
	created, err := dateRange(in.From, in.To)
	if err != nil {
		return nil, tiime.RawPage{}, err
	}

	filters := map[string]string{
		"status":   in.Status,
		"category": in.Category,
		"types":    in.EntityType,
		"date":     created,
	}
	if in.EntityID != 0 {
		if in.EntityType == "" {
			return nil, tiime.RawPage{}, errors.New("entity_id needs entity_type")
		}
		// The entity ID is sent under the name of its type.
		filters[in.EntityType] = strconv.FormatInt(in.EntityID, 10)
	}

	page, err := h.api.ExpertThreads(ctx, companyID, tiime.ExpertQuery{Search: in.Search, Filters: filters}, in.window())

	return nil, page, err
}

// CreateThread implements the create_thread tool.
func (h *ExpertHandler) CreateThread(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CreateThreadInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if in.EntityType == "" || in.EntityID == 0 {
		return nil, nil, errors.New("entity_type and entity_id are required")
	}
	if err := checkEnum("entity_type", in.EntityType, threadEntityTypes); err != nil {
		return nil, nil, err
	}
	if err := checkEnum("category", in.Category, threadCategories); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(in.Message) == "" {
		return nil, nil, errors.New("message is required")
	}

	thread := map[string]any{
		"entity":        map[string]any{"type": in.EntityType, in.EntityType: idRef(in.EntityID)},
		"interlocutors": []any{},
		"messages":      []map[string]string{{"content": in.Message}},
		"tags":          []any{},
	}
	if in.Category != "" {
		thread["category"] = in.Category
	}

	created, err := h.api.CreateExpertThread(ctx, companyID, thread)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(created), nil, nil
}

// PostThreadMessage implements the post_thread_message tool.
func (h *ExpertHandler) PostThreadMessage(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ThreadMessageInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, nil, err
	}
	if in.ThreadID == 0 {
		return nil, nil, errors.New("thread_id is required: see list_threads")
	}
	if strings.TrimSpace(in.Content) == "" {
		return nil, nil, errors.New("content is required")
	}

	message, err := h.api.PostExpertThreadMessage(ctx, companyID, in.ThreadID, in.Content)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(message), nil, nil
}

// UpdateThread implements the update_thread tool.
func (h *ExpertHandler) UpdateThread(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ThreadActionInput,
) (*mcp.CallToolResult, DoneOutput, error) {
	companyID, err := h.company(in.CompanyID)
	if err != nil {
		return nil, DoneOutput{}, err
	}
	if in.ThreadID == 0 {
		return nil, DoneOutput{}, errors.New("thread_id is required: see list_threads")
	}
	if in.Action == "" {
		return nil, DoneOutput{}, errors.New("action is required: read, close, unclose, archive or unarchive")
	}
	if err := checkEnum("action", in.Action, threadActions); err != nil {
		return nil, DoneOutput{}, err
	}

	if err := h.api.ExpertThreadAction(ctx, companyID, in.ThreadID, in.Action); err != nil {
		return nil, DoneOutput{}, err
	}

	return nil, DoneOutput{Done: true}, nil
}

// ListNotifications implements the list_notifications tool.
func (h *ExpertHandler) ListNotifications(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in NotificationsInput,
) (*mcp.CallToolResult, NotificationsOutput, error) {
	read := ""
	if in.UnreadOnly {
		read = "false"
	}

	page, err := h.api.Notifications(ctx, read, in.window())
	if err != nil {
		return nil, NotificationsOutput{}, err
	}
	out := NotificationsOutput{RawPage: page}

	count, err := h.api.NotificationCount(ctx)
	if err != nil {
		return nil, NotificationsOutput{}, err
	}
	if err := json.Unmarshal(count, &out.Unread); err != nil {
		return nil, NotificationsOutput{}, fmt.Errorf(
			"decoding notification count: %w",
			err,
		)
	}

	return nil, out, nil
}

// MarkNotificationsRead implements the mark_notifications_read tool.
func (h *ExpertHandler) MarkNotificationsRead(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in MarkNotificationsInput,
) (*mcp.CallToolResult, DoneOutput, error) {
	if in.All {
		if err := h.api.MarkAllNotificationsRead(ctx); err != nil {
			return nil, DoneOutput{}, err
		}
		return nil, DoneOutput{Done: true}, nil
	}
	if len(in.NotificationIDs) == 0 {
		return nil, DoneOutput{}, errors.New("give notification_ids or all")
	}

	for _, id := range in.NotificationIDs {
		if err := h.api.MarkNotificationRead(ctx, id); err != nil {
			return nil, DoneOutput{}, err
		}
	}

	return nil, DoneOutput{Done: true}, nil
}

func checkEnums(
	field string,
	values []string,
	allowed []string,
) error {
	for _, value := range values {
		if err := checkEnum(field, value, allowed); err != nil {
			return err
		}
	}

	return nil
}
