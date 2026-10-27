package tools

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// ListExpenseReportsInput selects expense reports.
type ListExpenseReportsInput struct {
	CompanyInput
	PageInput
}

// CreateExpenseReportInput describes an expense report.
type CreateExpenseReportInput struct {
	CompanyInput
	Name        string  `json:"name" jsonschema:"Name of the expense report, e.g. Expenses October 2026"`
	Date        string  `json:"date" jsonschema:"Date of the expense report, YYYY-MM-DD"`
	DocumentIDs []int64 `json:"document_ids" jsonschema:"IDs of the advanced expense documents to include, at least one as Tiime rejects empty expense reports. Upload them with upload_document and category advanced_expense"`
	Comment     string  `json:"comment,omitempty" jsonschema:"Comment on the expense report"`
	Submit      bool    `json:"submit,omitempty" jsonschema:"true to save the report, false (default) to keep it as a draft"`
}

// UpdateExpenseReportInput describes changes on an expense report.
type UpdateExpenseReportInput struct {
	CompanyInput
	ID                int64   `json:"id" jsonschema:"ID of the expense report"`
	Name              string  `json:"name,omitempty" jsonschema:"New name"`
	Date              string  `json:"date,omitempty" jsonschema:"New date, YYYY-MM-DD"`
	Comment           *string `json:"comment,omitempty" jsonschema:"New comment"`
	AddDocumentIDs    []int64 `json:"add_document_ids,omitempty" jsonschema:"Advanced expense documents to add"`
	RemoveDocumentIDs []int64 `json:"remove_document_ids,omitempty" jsonschema:"Advanced expense documents to remove"`
	Submit            bool    `json:"submit,omitempty" jsonschema:"true to save a draft report"`
}

// CreateTravelInput describes a trip.
type CreateTravelInput struct {
	CompanyInput
	Date               string        `json:"date" jsonschema:"Date of the trip, YYYY-MM-DD"`
	Distance           float64       `json:"distance" jsonschema:"Distance in km, one way"`
	VehicleID          int64         `json:"vehicle_id" jsonschema:"Vehicle used, see list_vehicles"`
	Departure          tiime.Address `json:"departure" jsonschema:"Departure address"`
	Arrival            tiime.Address `json:"arrival" jsonschema:"Arrival address"`
	ArrivalCompanyName string        `json:"arrival_company_name,omitempty" jsonschema:"Company visited"`
	RoundTrip          bool          `json:"round_trip,omitempty" jsonschema:"true for a round trip"`
	Comment            string        `json:"comment,omitempty" jsonschema:"Purpose of the trip"`
}

// EstimateTravelInput describes a distance to price.
type EstimateTravelInput struct {
	CompanyInput
	VehicleID int64   `json:"vehicle_id" jsonschema:"Vehicle used, see list_vehicles"`
	Distance  float64 `json:"distance" jsonschema:"Distance in km"`
}

// CreateMileageReportInput describes a mileage allowance expense report.
type CreateMileageReportInput struct {
	CompanyInput
	Name      string  `json:"name" jsonschema:"Name of the report, e.g. Mileage October 2026"`
	Date      string  `json:"date" jsonschema:"Date of the report, YYYY-MM-DD"`
	TravelIDs []int64 `json:"travel_ids" jsonschema:"Trips to include, see list_travels and create_travel"`
	Comment   string  `json:"comment,omitempty" jsonschema:"Comment on the report"`
	Submit    bool    `json:"submit,omitempty" jsonschema:"true to save the report, false (default) to keep it as a draft"`
}

// ListTravelsInput selects trips.
type ListTravelsInput struct {
	CompanyInput
	PageInput
}

func registerExpenses(
	server *mcp.Server,
	h *Handler,
) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_expense_reports",
		Description: "List the expense reports of the company.",
		Annotations: readOnly,
	}, h.ListExpenseReports)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_expense_report",
		Description: "Get the full detail of an expense report, including its expenses.",
		Annotations: readOnly,
	}, h.GetExpenseReport)

	mcp.AddTool(server, &mcp.Tool{
		Name: "create_expense_report",
		Description: "Create an expense report owned by the logged in user from advanced expense documents. " +
			"Upload each receipt first with upload_document, category advanced_expense, its date, amount and VAT.",
	}, h.CreateExpenseReport)

	destructive := true

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_expense_report",
		Description: "Update an expense report: rename it, change its date or comment, add or remove advanced expense documents, or submit a draft.",
	}, h.UpdateExpenseReport)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_expense_report",
		Description: "Delete an expense report that is not matched with a payment yet. Its expense documents are kept.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.DeleteExpenseReport)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_vehicles",
		Description: "List the vehicles used for mileage allowances.",
		Annotations: readOnly,
	}, h.ListVehicles)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_travels",
		Description: "List the trips recorded for mileage allowances, most recent first.",
		Annotations: readOnly,
	}, h.ListTravels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_travel",
		Description: "Record a business trip for mileage allowances.",
	}, h.CreateTravel)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "estimate_travel_amount",
		Description: "Compute the mileage allowance of a distance with a vehicle, using the official scale.",
		Annotations: readOnly,
	}, h.EstimateTravelAmount)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_mileage_report",
		Description: "Create a mileage allowance expense report from recorded trips, owned by the logged in user.",
	}, h.CreateMileageReport)
}

// UpdateExpenseReport implements the update_expense_report tool.
func (h *Handler) UpdateExpenseReport(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in UpdateExpenseReportInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	report, err := h.api.UpdateExpenseReport(ctx, companyID, in.ID, tiime.ExpenseReportUpdate{
		Name:              in.Name,
		Date:              in.Date,
		Comment:           in.Comment,
		AddDocumentIDs:    in.AddDocumentIDs,
		RemoveDocumentIDs: in.RemoveDocumentIDs,
		Submit:            in.Submit,
	})
	if err != nil {
		return nil, nil, err
	}

	return rawResult(report), nil, nil
}

// DeleteExpenseReport implements the delete_expense_report tool. Expense
// reports are documents of the documents space.
func (h *Handler) DeleteExpenseReport(
	ctx context.Context,
	request *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, DeleteOutput, error) {
	return h.DeleteDocument(ctx, request, in)
}

// ListVehicles implements the list_vehicles tool.
func (h *Handler) ListVehicles(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CompanyInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	vehicles, err := h.api.Vehicles(ctx, companyID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(vehicles), nil, nil
}

// ListTravels implements the list_travels tool.
func (h *Handler) ListTravels(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListTravelsInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	page, err := h.api.Travels(ctx, companyID, in.window())
	if err != nil {
		return nil, nil, err
	}

	return jsonResult(page)
}

// CreateTravel implements the create_travel tool.
func (h *Handler) CreateTravel(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CreateTravelInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	travel, err := h.api.CreateTravel(ctx, companyID, tiime.TravelInput{
		Date:               in.Date,
		Distance:           in.Distance,
		VehicleID:          in.VehicleID,
		Departure:          in.Departure,
		Arrival:            in.Arrival,
		ArrivalCompanyName: in.ArrivalCompanyName,
		RoundTrip:          in.RoundTrip,
		Comment:            in.Comment,
	})
	if err != nil {
		return nil, nil, err
	}

	return rawResult(travel), nil, nil
}

// EstimateTravelAmount implements the estimate_travel_amount tool.
func (h *Handler) EstimateTravelAmount(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in EstimateTravelInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	estimate, err := h.api.EstimateTravelAmount(ctx, companyID, in.VehicleID, in.Distance)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(estimate), nil, nil
}

// CreateMileageReport implements the create_mileage_report tool.
func (h *Handler) CreateMileageReport(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CreateMileageReportInput,
) (*mcp.CallToolResult, any, error) {
	if len(in.TravelIDs) == 0 {
		return nil, nil, errRequired("at least one travel_ids entry is")
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	report, err := h.api.CreateMileageReport(ctx, companyID, tiime.ExpenseReportInput{
		Name:      in.Name,
		Date:      in.Date,
		TravelIDs: in.TravelIDs,
		Comment:   in.Comment,
		Submit:    in.Submit,
	})
	if err != nil {
		return nil, nil, err
	}

	return rawResult(report), nil, nil
}

// ListExpenseReports implements the list_expense_reports tool.
func (h *Handler) ListExpenseReports(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListExpenseReportsInput,
) (*mcp.CallToolResult, tiime.Page[tiime.ExpenseReport], error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.Page[tiime.ExpenseReport]{}, err
	}

	page, err := h.api.ExpenseReports(ctx, companyID, in.window())

	return nil, page, err
}

// GetExpenseReport implements the get_expense_report tool.
func (h *Handler) GetExpenseReport(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	report, err := h.api.ExpenseReport(ctx, companyID, in.ID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(report), nil, nil
}

// CreateExpenseReport implements the create_expense_report tool.
func (h *Handler) CreateExpenseReport(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CreateExpenseReportInput,
) (*mcp.CallToolResult, any, error) {
	if len(in.DocumentIDs) == 0 {
		return nil, nil, errors.New("tiime rejects empty expense reports: upload at least one expense with upload_document (category advanced_expense) and pass its ID in document_ids")
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	report, err := h.api.CreateExpenseReport(ctx, companyID, tiime.ExpenseReportInput{
		Name:        in.Name,
		Date:        in.Date,
		DocumentIDs: in.DocumentIDs,
		Comment:     in.Comment,
		Submit:      in.Submit,
	})
	if err != nil {
		return nil, nil, err
	}

	return rawResult(report), nil, nil
}
