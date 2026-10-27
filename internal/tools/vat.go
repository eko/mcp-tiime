package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// VATAPI reads the VAT computed by Tiime.
type VATAPI interface {
	VATEstimate(
		ctx context.Context,
		companyID int64,
		year int,
		month int,
	) (tiime.VATEstimate, error)
	VATMovements(
		ctx context.Context,
		companyID int64,
		year int,
		month int,
		kind string,
		window tiime.Range,
	) (tiime.Page[tiime.VATMovement], error)
}

// VATDetailsInput selects the entries behind a VAT amount.
type VATDetailsInput struct {
	VATEstimateInput
	PageInput
	Kind string `json:"kind" jsonschema:"collected or deductible"`
}

// VATEstimateInput selects the month of a VAT estimate.
type VATEstimateInput struct {
	CompanyInput
	Month string `json:"month,omitempty" jsonschema:"Month, YYYY-MM, defaults to the current month"`
}

func registerVAT(
	server *mcp.Server,
	h *Handler,
) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "get_vat_estimate",
		Description: "Get the VAT Tiime computes for a month: collected and deductible VAT, amount due, payment deadline " +
			"and declaration status. Use the current month to anticipate the next VAT payment.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, h.GetVATEstimate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_vat_details",
		Description: "List the entries (bank transactions, receipts, invoices, adjustments) behind the collected or deductible VAT of a month.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, h.GetVATDetails)
}

// GetVATEstimate implements the get_vat_estimate tool.
func (h *Handler) GetVATEstimate(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VATEstimateInput,
) (*mcp.CallToolResult, tiime.VATEstimate, error) {
	month, err := parseMonth(in.Month)
	if err != nil {
		return nil, tiime.VATEstimate{}, err
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.VATEstimate{}, err
	}

	estimate, err := h.api.VATEstimate(ctx, companyID, month.Year(), int(month.Month()))

	return nil, estimate, err
}

// GetVATDetails implements the get_vat_details tool.
func (h *Handler) GetVATDetails(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VATDetailsInput,
) (*mcp.CallToolResult, tiime.Page[tiime.VATMovement], error) {
	if in.Kind != "collected" && in.Kind != "deductible" {
		return nil, tiime.Page[tiime.VATMovement]{}, fmt.Errorf("kind must be collected or deductible, got %q", in.Kind)
	}

	month, err := parseMonth(in.Month)
	if err != nil {
		return nil, tiime.Page[tiime.VATMovement]{}, err
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.Page[tiime.VATMovement]{}, err
	}

	page, err := h.api.VATMovements(ctx, companyID, month.Year(), int(month.Month()), in.Kind, in.window())

	return nil, page, err
}

// parseMonth reads a YYYY-MM month, the current month when empty.
func parseMonth(
	value string,
) (time.Time, error) {
	if value == "" {
		return time.Now(), nil
	}

	month, err := time.Parse("2006-01", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month %q, expected YYYY-MM", value)
	}

	return month, nil
}
