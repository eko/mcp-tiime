package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// InvoiceDraftInput describes an invoice draft.
type InvoiceDraftInput struct {
	CompanyInput
	ClientID     int64               `json:"client_id,omitempty" jsonschema:"Client to invoice, see list_clients. Required on creation"`
	EmissionDate string              `json:"emission_date,omitempty" jsonschema:"Emission date, YYYY-MM-DD. Required on creation"`
	DueDate      string              `json:"due_date,omitempty" jsonschema:"Due date, YYYY-MM-DD. Defaults to the payment terms of the latest invoice"`
	Title        string              `json:"title,omitempty" jsonschema:"Invoice title"`
	Comment      string              `json:"comment,omitempty" jsonschema:"Internal comment"`
	Lines        []tiime.InvoiceLine `json:"lines,omitempty" jsonschema:"Invoice lines. Required on creation, replaces all lines on update"`
}

// UpdateInvoiceDraftInput describes changes on an invoice draft.
type UpdateInvoiceDraftInput struct {
	InvoiceDraftInput
	ID int64 `json:"id" jsonschema:"ID of the draft invoice"`
}

// UpdateClientInput describes changes on a client.
type UpdateClientInput struct {
	CompanyInput
	tiime.CustomerInput
	ID int64 `json:"id" jsonschema:"ID of the client"`
}

// CreateClientInput describes a new client.
type CreateClientInput struct {
	CompanyInput
	tiime.CustomerInput
}

// VATTypesOutput lists VAT types.
type VATTypesOutput struct {
	VATTypes []tiime.VATType `json:"vat_types"`
}

func registerInvoicingWrites(
	server *mcp.Server,
	h *Handler,
) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_vat_types",
		Description: "List the VAT type codes usable on invoice lines, with their rates.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, h.ListVATTypes)

	mcp.AddTool(server, &mcp.Tool{
		Name: "create_invoice_draft",
		Description: "Create a draft sales invoice for a client. Layout, seller details, payment terms and bank details are copied from " +
			"the latest invoice. The draft must be reviewed and sent from Tiime.",
	}, h.CreateInvoiceDraft)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_invoice_draft",
		Description: "Update a draft sales invoice: client, dates, title or lines. Omitted fields are left unchanged.",
	}, h.UpdateInvoiceDraft)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_invoice_from_quote",
		Description: "Create an invoice from an accepted quote.",
	}, h.CreateInvoiceFromQuote)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_client",
		Description: "Create an invoicing client. Defaults to a French company.",
	}, h.CreateClient)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_client",
		Description: "Update an invoicing client. Omitted fields are left unchanged.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.UpdateClient)
}

// ListVATTypes implements the list_vat_types tool.
func (h *Handler) ListVATTypes(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ struct{},
) (*mcp.CallToolResult, VATTypesOutput, error) {
	types, err := h.api.VATTypes(ctx)

	return nil, VATTypesOutput{VATTypes: types}, err
}

// CreateInvoiceDraft implements the create_invoice_draft tool.
func (h *Handler) CreateInvoiceDraft(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in InvoiceDraftInput,
) (*mcp.CallToolResult, any, error) {
	if in.ClientID == 0 || in.EmissionDate == "" || len(in.Lines) == 0 {
		return nil, nil, errRequired("client_id, emission_date and lines")
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	invoice, err := h.api.CreateInvoiceDraft(ctx, companyID, in.draft())
	if err != nil {
		return nil, nil, err
	}

	return rawResult(invoice), nil, nil
}

// UpdateInvoiceDraft implements the update_invoice_draft tool.
func (h *Handler) UpdateInvoiceDraft(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in UpdateInvoiceDraftInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	invoice, err := h.api.UpdateInvoiceDraft(ctx, companyID, in.ID, in.draft())
	if err != nil {
		return nil, nil, err
	}

	return rawResult(invoice), nil, nil
}

// CreateInvoiceFromQuote implements the create_invoice_from_quote tool.
func (h *Handler) CreateInvoiceFromQuote(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	invoice, err := h.api.InvoiceFromQuote(ctx, companyID, in.ID)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(invoice), nil, nil
}

// CreateClient implements the create_client tool.
func (h *Handler) CreateClient(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CreateClientInput,
) (*mcp.CallToolResult, any, error) {
	if in.Name == nil || *in.Name == "" {
		return nil, nil, errRequired("name")
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	client, err := h.api.CreateCustomer(ctx, companyID, in.CustomerInput)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(client), nil, nil
}

// UpdateClient implements the update_client tool.
func (h *Handler) UpdateClient(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in UpdateClientInput,
) (*mcp.CallToolResult, any, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, nil, err
	}

	client, err := h.api.UpdateCustomer(ctx, companyID, in.ID, in.CustomerInput)
	if err != nil {
		return nil, nil, err
	}

	return rawResult(client), nil, nil
}

func (in InvoiceDraftInput) draft() tiime.InvoiceDraft {
	return tiime.InvoiceDraft{
		ClientID:     in.ClientID,
		EmissionDate: in.EmissionDate,
		DueDate:      in.DueDate,
		Title:        in.Title,
		Comment:      in.Comment,
		Lines:        in.Lines,
	}
}

func errRequired(
	fields string,
) error {
	return fmt.Errorf("%s required", fields)
}
