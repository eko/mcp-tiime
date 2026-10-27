package tiime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DashboardKeys are the dashboard tiles useful outside the web application.
var DashboardKeys = []string{
	"in_out_evolution", "total_turnover_amounts", "total_sales_amounts", "clients_total_due_amount",
	"to_do_counts", "count_unread_thread", "month_to_declare", "has_estimated_vat_to_declare",
	"update_needed_bank_connections", "scheduled_invoices",
}

// Dashboard returns the dashboard tiles of the web application.
func (c *Client) Dashboard(
	ctx context.Context,
	companyID int64,
	keys []string,
) (json.RawMessage, error) {
	query := url.Values{"keys": {strings.Join(keys, ",")}}
	headers := map[string]string{"Accept": "application/vnd.tiime.tile.todo.v3+json"}

	var raw json.RawMessage
	_, err := c.getJSON(ctx, accountsPath(companyID, "/tiles"), query, headers, &raw)

	return raw, err
}

// RevenueBook lists the entries of the revenue book.
func (c *Client) RevenueBook(
	ctx context.Context,
	companyID int64,
	window Range,
) (Page[json.RawMessage], error) {
	return getPage[json.RawMessage](ctx, c, "/invoice"+companyPath(companyID, "/registries"), nil, nil, window, "")
}

// ExportRevenueBook downloads the revenue book as csv or pdf.
func (c *Client) ExportRevenueBook(
	ctx context.Context,
	companyID int64,
	format string,
) ([]byte, error) {
	resp, err := c.do(ctx, request{
		method:  http.MethodGet,
		path:    "/invoice" + companyPath(companyID, "/registries/"+url.PathEscape(format)),
		headers: map[string]string{"Accept": "*/*", "Content-Type": "application/" + format},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	return io.ReadAll(resp.Body)
}

// ForecastRow is a line of the cash forecast, with a value per month.
type ForecastRow struct {
	ID int64 `json:"id,omitempty"`
	// Name labels the row, e.g. "Rent".
	Name string `json:"name" jsonschema:"Name of the forecast row, e.g. Rent"`
	// FlowType is "in" for inflows or "out" for outflows.
	FlowType       string          `json:"flow_type" jsonschema:"in for inflows, out for outflows"`
	IntervalValues []ForecastValue `json:"interval_values" jsonschema:"Amount per month"`
}

// ForecastValue is the amount of a forecast row for a month.
type ForecastValue struct {
	Date  string   `json:"date" jsonschema:"First day of the month, YYYY-MM-DD"`
	Value *float64 `json:"value" jsonschema:"Amount, null for none"`
}

// ForecastEnabled reports whether the cash forecast is activated.
func (c *Client) ForecastEnabled(
	ctx context.Context,
	companyID int64,
) (bool, error) {
	var status struct {
		ActivatedAt string `json:"activated_at"`
	}
	resp, err := c.do(ctx, request{method: http.MethodGet, path: accountsPath(companyID, "/forecast_balance")})
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return false, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}

	return status.ActivatedAt != "", nil
}

// SetForecastEnabled activates or deactivates the cash forecast.
func (c *Client) SetForecastEnabled(
	ctx context.Context,
	companyID int64,
	enabled bool,
) error {
	var activatedAt any
	if enabled {
		activatedAt = time.Now().Format(time.DateTime)
	}

	return c.sendJSON(ctx, http.MethodPatch, accountsPath(companyID, "/forecast_balance"), map[string]any{"activated_at": activatedAt}, nil)
}

// ForecastRows lists the forecast rows from a month, formatted as YYYY-MM-DD.
func (c *Client) ForecastRows(
	ctx context.Context,
	companyID int64,
	startDate string,
) ([]ForecastRow, error) {
	var rows []ForecastRow
	_, err := c.getJSON(ctx, accountsPath(companyID, "/forecast_balance_rows"), url.Values{"start_date": {startDate}}, nil, &rows)

	return rows, err
}

// SaveForecastRow creates a forecast row, or updates it when row.ID is set.
func (c *Client) SaveForecastRow(
	ctx context.Context,
	companyID int64,
	row ForecastRow,
) (ForecastRow, error) {
	path := accountsPath(companyID, "/forecast_balance_rows")
	method := http.MethodPost
	if row.ID != 0 {
		path += "/" + strconv.FormatInt(row.ID, 10)
		method = http.MethodPatch
	}

	var saved ForecastRow
	err := c.sendJSON(ctx, method, path, row, &saved)

	return saved, err
}

// DeleteForecastRow deletes a forecast row.
func (c *Client) DeleteForecastRow(
	ctx context.Context,
	companyID int64,
	rowID int64,
) error {
	_, err := c.call(ctx, request{method: http.MethodDelete, path: accountsPath(companyID, "/forecast_balance_rows/"+strconv.FormatInt(rowID, 10))}, nil)

	return err
}
