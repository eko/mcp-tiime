package tiime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const advancedExpenseType = "advanced_expense"

// expandPreview mirrors the query parameter sent by the web application on
// every expense report call.
var expandPreview = url.Values{"expand": {"preview_available"}}

// ExpenseReports lists the expense reports of a company, most recent first.
func (c *Client) ExpenseReports(
	ctx context.Context,
	companyID int64,
	window Range,
) (Page[ExpenseReport], error) {
	// Without explicit sort, Tiime returns the oldest reports first.
	query := url.Values{"expand": {"preview_available"}, "sorts": {"date:desc"}}

	page, err := getPage[ExpenseReport](ctx, c, accountsPath(companyID, "/expense_reports"), query, nil, window, "expense_reports")
	if err != nil {
		return page, err
	}

	for i, report := range page.Items {
		for _, entry := range report.Metadata {
			switch entry.Key {
			case "amount":
				page.Items[i].Amount = amountValue(entry.Value)
			case "vat_amount":
				page.Items[i].VATAmount = amountValue(vatTotal(entry.Value))
			}
		}
		page.Items[i].Metadata = nil
	}

	return page, nil
}

// ExpenseReport returns the full detail of an expense report.
func (c *Client) ExpenseReport(
	ctx context.Context,
	companyID int64,
	reportID int64,
) (json.RawMessage, error) {
	var raw json.RawMessage
	_, err := c.getJSON(ctx, expenseReportPath(companyID, reportID), expandPreview, nil, &raw)

	return raw, err
}

// CreateExpenseReport creates an expense report from advanced expense
// documents, owned by the authenticated user.
func (c *Client) CreateExpenseReport(
	ctx context.Context,
	companyID int64,
	input ExpenseReportInput,
) (json.RawMessage, error) {
	var owner map[string]any
	if _, err := c.getJSON(ctx, "/users/me", nil, nil, &owner); err != nil {
		return nil, fmt.Errorf(
			"loading current user: %w",
			err,
		)
	}

	expenses := make([]map[string]any, 0, len(input.DocumentIDs))
	for _, documentID := range input.DocumentIDs {
		document, err := c.AccountingDocument(ctx, companyID, documentID)
		if err != nil {
			return nil, fmt.Errorf(
				"loading document %d: %w",
				documentID,
				err,
			)
		}
		expenses = append(expenses, advancedExpense(document, owner))
	}

	status := "draft"
	if input.Submit {
		status = "saved"
	}

	payload := map[string]any{
		"name":              input.Name,
		"date":              input.Date,
		"owner":             ownerRef(owner),
		"expense_type":      advancedExpenseType,
		"advanced_expenses": expenses,
		"travels":           []any{},
		"tags":              []any{},
		"comment":           input.Comment,
		"lifecycle_status":  status,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var raw json.RawMessage
	_, err = c.call(ctx, request{
		method:      http.MethodPost,
		path:        accountsPath(companyID, "/expense_reports"),
		query:       expandPreview,
		body:        body,
		contentType: "application/json",
	}, &raw)

	return raw, err
}

// UpdateExpenseReport changes an expense report: name, date, comment,
// advanced expenses, or submits a draft. Tiime expects the full report.
func (c *Client) UpdateExpenseReport(
	ctx context.Context,
	companyID int64,
	reportID int64,
	update ExpenseReportUpdate,
) (json.RawMessage, error) {
	var report map[string]any
	if _, err := c.getJSON(ctx, expenseReportPath(companyID, reportID), expandPreview, nil, &report); err != nil {
		return nil, err
	}

	owner, _ := report["owner"].(map[string]any)

	payload := map[string]any{}
	for _, key := range []string{"id", "name", "date", "owner", "comment", "tags", "label", "payment_status", "metadata", "expense_type", "travels", "lifecycle_status"} {
		payload[key] = report[key]
	}
	if update.Name != "" {
		payload["name"] = update.Name
	}
	if update.Date != "" {
		payload["date"] = update.Date
	}
	if update.Comment != nil {
		payload["comment"] = *update.Comment
	}
	if update.Submit {
		payload["lifecycle_status"] = "saved"
	}

	expenses := []map[string]any{}
	if report["expense_type"] == advancedExpenseType {
		current, _ := report["advanced_expenses"].([]any)
		for _, item := range current {
			expense, ok := item.(map[string]any)
			if !ok || containsID(update.RemoveDocumentIDs, expense["id"]) {
				continue
			}
			expenses = append(expenses, existingExpense(expense))
		}
		for _, documentID := range update.AddDocumentIDs {
			document, err := c.AccountingDocument(ctx, companyID, documentID)
			if err != nil {
				return nil, fmt.Errorf(
					"loading document %d: %w",
					documentID,
					err,
				)
			}
			expenses = append(expenses, advancedExpense(document, owner))
		}
	}
	payload["advanced_expenses"] = expenses

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var raw json.RawMessage
	_, err = c.call(ctx, request{
		method:      http.MethodPatch,
		path:        expenseReportPath(companyID, reportID),
		query:       expandPreview,
		body:        body,
		contentType: "application/json",
	}, &raw)

	return raw, err
}

// Vehicles lists the vehicles used for mileage allowances.
func (c *Client) Vehicles(
	ctx context.Context,
	companyID int64,
) (json.RawMessage, error) {
	var body struct {
		Vehicles json.RawMessage `json:"vehicles"`
	}
	headers := map[string]string{"Accept": "application/vnd.tiime.vehicles.v2+json", "Range": "items=0-99"}
	_, err := c.getJSON(ctx, accountsPath(companyID, "/vehicles"), nil, headers, &body)

	return body.Vehicles, err
}

// Travels lists the trips recorded for mileage allowances.
func (c *Client) Travels(
	ctx context.Context,
	companyID int64,
	window Range,
) (Page[map[string]any], error) {
	query := url.Values{"with_total_distance": {"true"}, "has_existing_travels": {"true"}, "sorts": {"date:desc"}}

	return getPage[map[string]any](ctx, c, accountsPath(companyID, "/travels"), query, nil, window, "travels")
}

// CreateTravel records a trip for mileage allowances.
func (c *Client) CreateTravel(
	ctx context.Context,
	companyID int64,
	input TravelInput,
) (json.RawMessage, error) {
	payload := map[string]any{
		"date":                 input.Date,
		"distance":             input.Distance,
		"vehicle_id":           input.VehicleID,
		"departure_address":    input.Departure,
		"arrival_address":      input.Arrival,
		"arrival_company_name": input.ArrivalCompanyName,
		"round_trip":           input.RoundTrip,
		"comment":              input.Comment,
		"tags":                 []any{},
	}

	var raw json.RawMessage
	err := c.sendJSON(ctx, http.MethodPost, accountsPath(companyID, "/travels"), payload, &raw)

	return raw, err
}

// EstimateTravelAmount returns the mileage allowance of a distance with a vehicle.
func (c *Client) EstimateTravelAmount(
	ctx context.Context,
	companyID int64,
	vehicleID int64,
	distance float64,
) (json.RawMessage, error) {
	query := url.Values{
		"vehicle_id": {strconv.FormatInt(vehicleID, 10)},
		"distance":   {strconv.FormatFloat(distance, 'f', -1, 64)},
	}

	var raw json.RawMessage
	_, err := c.getJSON(ctx, accountsPath(companyID, "/travels/estimated_amount"), query, nil, &raw)

	return raw, err
}

// CreateMileageReport creates a mileage allowance expense report from
// recorded trips, owned by the authenticated user.
func (c *Client) CreateMileageReport(
	ctx context.Context,
	companyID int64,
	input ExpenseReportInput,
) (json.RawMessage, error) {
	var owner map[string]any
	if _, err := c.getJSON(ctx, "/users/me", nil, nil, &owner); err != nil {
		return nil, fmt.Errorf(
			"loading current user: %w",
			err,
		)
	}

	travels, err := c.findTravels(ctx, companyID, input.TravelIDs)
	if err != nil {
		return nil, err
	}

	status := "draft"
	if input.Submit {
		status = "saved"
	}

	payload := map[string]any{
		"name":              input.Name,
		"date":              input.Date,
		"owner":             ownerRef(owner),
		"expense_type":      "travel",
		"advanced_expenses": []any{},
		"travels":           travels,
		"tags":              []any{},
		"comment":           input.Comment,
		"lifecycle_status":  status,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var raw json.RawMessage
	_, err = c.call(ctx, request{
		method:      http.MethodPost,
		path:        accountsPath(companyID, "/expense_reports"),
		query:       expandPreview,
		body:        body,
		contentType: "application/json",
	}, &raw)

	return raw, err
}

// findTravels loads the trips with the given IDs from the recent trips.
func (c *Client) findTravels(
	ctx context.Context,
	companyID int64,
	travelIDs []int64,
) ([]map[string]any, error) {
	found := map[int64]map[string]any{}
	for offset := 0; offset < 1000 && len(found) < len(travelIDs); offset += 100 {
		page, err := c.Travels(ctx, companyID, Range{Offset: offset, Limit: 100})
		if err != nil {
			return nil, err
		}
		for _, travel := range page.Items {
			if id, ok := travel["id"].(float64); ok && containsID(travelIDs, id) {
				found[int64(id)] = travel
			}
		}
		if !page.HasMore {
			break
		}
	}

	travels := make([]map[string]any, 0, len(travelIDs))
	for _, id := range travelIDs {
		travel, ok := found[id]
		if !ok {
			return nil, fmt.Errorf("trip %d not found, see list_travels", id)
		}
		travels = append(travels, travel)
	}

	return travels, nil
}

// existingExpense keeps the fields of an expense report line that Tiime
// expects back when the report is saved.
func existingExpense(
	expense map[string]any,
) map[string]any {
	line := map[string]any{}
	for _, key := range []string{"id", "amount", "currency", "date", "vat_amount", "label", "comment", "owner", "name"} {
		if value, ok := expense[key]; ok {
			line[key] = value
		}
	}

	return line
}

func containsID(
	ids []int64,
	value any,
) bool {
	id, ok := value.(float64)
	if !ok {
		return false
	}

	for _, candidate := range ids {
		if float64(candidate) == id {
			return true
		}
	}

	return false
}

// advancedExpense builds an expense report line from a document, the way the
// web application reads its metadata.
func advancedExpense(
	document AccountingDocument,
	owner map[string]any,
) map[string]any {
	expense := map[string]any{
		"id":       document.ID,
		"name":     document.Name,
		"comment":  document.Comment,
		"currency": "EUR",
		"owner":    ownerRef(owner),
	}
	if document.Label != nil {
		expense["label"] = document.Label
	}

	for _, entry := range document.Metadata {
		switch entry.Key {
		case "date":
			expense["date"] = entry.Value
		case "amount":
			if amount, ok := entry.Value.(map[string]any); ok {
				expense["amount"] = amount["value"]
				if currency, ok := amount["currency"].(string); ok && currency != "" {
					expense["currency"] = currency
				}
			}
		case "vat_amount":
			if vat := amountPointer(vatTotal(entry.Value)); vat != nil {
				expense["vat_amount"] = *vat
			}
		}
	}

	return expense
}

// amountValue reads the value of an amount stored as {"value": x}, or x itself.
func amountValue(
	value any,
) float64 {
	if amount := amountPointer(value); amount != nil {
		return *amount
	}

	return 0
}

// amountPointer is amountValue returning nil when no amount is set.
func amountPointer(
	value any,
) *float64 {
	if amount, ok := value.(map[string]any); ok {
		value = amount["value"]
	}
	number, ok := value.(float64)
	if !ok {
		return nil
	}

	return &number
}

// vatTotal reads a VAT amount stored either as {"value": x} or as a list of
// per-rate amounts holding a "total" entry, whose own value may be null.
func vatTotal(
	value any,
) any {
	switch typed := value.(type) {
	case map[string]any:
		return typed["value"]
	case []any:
		for _, item := range typed {
			entry, ok := item.(map[string]any)
			if !ok || entry["key"] != "total" {
				continue
			}
			return entry["value"]
		}
	}

	return nil
}

func ownerRef(
	owner map[string]any,
) map[string]any {
	ref := map[string]any{}
	for _, key := range []string{"id", "firstname", "lastname", "email"} {
		if value, ok := owner[key]; ok {
			ref[key] = value
		}
	}

	return ref
}

func expenseReportPath(
	companyID int64,
	reportID int64,
) string {
	return accountsPath(companyID, fmt.Sprintf("/expense_reports/%d", reportID))
}
