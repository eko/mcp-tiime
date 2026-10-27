package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// InsightsAPI reads the dashboard, the revenue book and the cash forecast.
type InsightsAPI interface {
	Dashboard(
		ctx context.Context,
		companyID int64,
		keys []string,
	) (json.RawMessage, error)
	RevenueBook(
		ctx context.Context,
		companyID int64,
		window tiime.Range,
	) (tiime.Page[json.RawMessage], error)
	ExportRevenueBook(
		ctx context.Context,
		companyID int64,
		format string,
	) ([]byte, error)
	ForecastEnabled(
		ctx context.Context,
		companyID int64,
	) (bool, error)
	SetForecastEnabled(
		ctx context.Context,
		companyID int64,
		enabled bool,
	) error
	ForecastRows(
		ctx context.Context,
		companyID int64,
		startDate string,
	) ([]tiime.ForecastRow, error)
	SaveForecastRow(
		ctx context.Context,
		companyID int64,
		row tiime.ForecastRow,
	) (tiime.ForecastRow, error)
	DeleteForecastRow(
		ctx context.Context,
		companyID int64,
		rowID int64,
	) error
}

// RevenueBookInput selects revenue book entries.
type RevenueBookInput struct {
	CompanyInput
	PageInput
}

// ExportInput selects an export format.
type ExportInput struct {
	CompanyInput
	Format string `json:"format,omitempty" jsonschema:"csv (default) or pdf"`
}

// ForecastInput selects the forecast period.
type ForecastInput struct {
	CompanyInput
	From string `json:"from,omitempty" jsonschema:"First month, YYYY-MM, defaults to the current month"`
}

// ForecastEnabledInput activates or deactivates the forecast.
type ForecastEnabledInput struct {
	CompanyInput
	Enabled bool `json:"enabled" jsonschema:"true to activate the cash forecast, false to deactivate it"`
}

// SaveForecastRowInput describes a forecast row.
type SaveForecastRowInput struct {
	CompanyInput
	tiime.ForecastRow
}

// ForecastOutput is the cash forecast.
type ForecastOutput struct {
	Enabled bool                `json:"enabled"`
	Rows    []tiime.ForecastRow `json:"rows"`
}

func registerInsights(
	server *mcp.Server,
	h *Handler,
) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := true

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_dashboard",
		Description: "Get the Tiime dashboard: inflows, outflows and bank balance of the month compared with last month, " +
			"turnover, receivables, to-do counts, next VAT month and bank connections to renew.",
		Annotations: readOnly,
	}, h.GetDashboard)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_revenue_book",
		Description: "List the entries of the revenue book.",
		Annotations: readOnly,
	}, h.ListRevenueBook)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "export_revenue_book",
		Description: "Download the revenue book as CSV or PDF to the local download directory.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.ExportRevenueBook)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_forecast",
		Description: "Get the cash forecast rows (expected inflows and outflows per month), if the forecast is activated.",
		Annotations: readOnly,
	}, h.GetForecast)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_forecast_enabled",
		Description: "Activate or deactivate the cash forecast of the company.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.SetForecastEnabled)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "save_forecast_row",
		Description: "Create a cash forecast row, e.g. an expected monthly rent, or update it when id is set.",
	}, h.SaveForecastRow)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_forecast_row",
		Description: "Delete a cash forecast row.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.DeleteForecastRow)
}

// GetDashboard implements the get_dashboard tool.
func (h *Handler) GetDashboard(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CompanyInput,
) (*mcp.CallToolResult, any, error) {
	return h.rawCall(ctx, in.CompanyID, func(companyID int64) (json.RawMessage, error) {
		return h.api.Dashboard(ctx, companyID, tiime.DashboardKeys)
	})
}

// ListRevenueBook implements the list_revenue_book tool.
func (h *Handler) ListRevenueBook(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in RevenueBookInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	page, err := h.api.RevenueBook(ctx, companyID, in.window())
	if err != nil {
		return nil, nil, err
	}

	return jsonResult(page)
}

// ExportRevenueBook implements the export_revenue_book tool.
func (h *Handler) ExportRevenueBook(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ExportInput,
) (*mcp.CallToolResult, DownloadOutput, error) {
	format := in.Format
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "pdf" {
		return nil, DownloadOutput{}, fmt.Errorf("format must be csv or pdf, got %q", format)
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, DownloadOutput{}, err
	}

	content, err := h.api.ExportRevenueBook(ctx, companyID, format)
	if err != nil {
		return nil, DownloadOutput{}, err
	}

	path, err := h.save(fmt.Sprintf("tiime-revenue-book-%s.%s", time.Now().Format(time.DateOnly), format), content)
	if err != nil {
		return nil, DownloadOutput{}, err
	}

	return nil, DownloadOutput{Path: path, Bytes: len(content)}, nil
}

// GetForecast implements the get_forecast tool.
func (h *Handler) GetForecast(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ForecastInput,
) (*mcp.CallToolResult, ForecastOutput, error) {
	month, err := parseMonth(in.From)
	if err != nil {
		return nil, ForecastOutput{}, err
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, ForecastOutput{}, err
	}

	enabled, err := h.api.ForecastEnabled(ctx, companyID)
	if err != nil || !enabled {
		return nil, ForecastOutput{Rows: []tiime.ForecastRow{}}, err
	}

	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)
	rows, err := h.api.ForecastRows(ctx, companyID, start)

	return nil, ForecastOutput{Enabled: true, Rows: rows}, err
}

// SetForecastEnabled implements the set_forecast_enabled tool.
func (h *Handler) SetForecastEnabled(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ForecastEnabledInput,
) (*mcp.CallToolResult, ForecastOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, ForecastOutput{}, err
	}

	if err := h.api.SetForecastEnabled(ctx, companyID, in.Enabled); err != nil {
		return nil, ForecastOutput{}, err
	}

	return nil, ForecastOutput{Enabled: in.Enabled, Rows: []tiime.ForecastRow{}}, nil
}

// SaveForecastRow implements the save_forecast_row tool.
func (h *Handler) SaveForecastRow(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in SaveForecastRowInput,
) (*mcp.CallToolResult, tiime.ForecastRow, error) {
	if in.FlowType != "in" && in.FlowType != "out" {
		return nil, tiime.ForecastRow{}, fmt.Errorf("flow_type must be in or out, got %q", in.FlowType)
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.ForecastRow{}, err
	}

	row, err := h.api.SaveForecastRow(ctx, companyID, in.ForecastRow)

	return nil, row, err
}

// DeleteForecastRow implements the delete_forecast_row tool.
func (h *Handler) DeleteForecastRow(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, DeleteOutput, error) {
	return h.deleteCall(ctx, in, h.api.DeleteForecastRow)
}
