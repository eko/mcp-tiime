package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestInputValidation checks that tools reject invalid input before calling
// Tiime: fakeAPI panics on any unexpected call.
func TestInputValidation(
	t *testing.T,
) {
	testCases := []struct {
		name      string
		tool      string
		arguments map[string]any
	}{
		{name: "quote status", tool: "update_quote", arguments: map[string]any{"id": 1, "status": "sent"}},
		{name: "payment amount", tool: "record_invoice_payment", arguments: map[string]any{"invoice_id": 1, "date": "2026-10-27", "amount": 0}},
		{name: "cancel date", tool: "cancel_invoice", arguments: map[string]any{"id": 1}},
		{name: "article code", tool: "save_article", arguments: map[string]any{"unit_amount": 700}},
		{name: "forecast flow type", tool: "save_forecast_row", arguments: map[string]any{"name": "Rent", "flow_type": "x", "interval_values": []any{}}},
		{name: "VAT details kind", tool: "get_vat_details", arguments: map[string]any{"kind": "total"}},
		{name: "revenue book format", tool: "export_revenue_book", arguments: map[string]any{"format": "xlsx"}},
		{name: "quote without lines", tool: "create_quote", arguments: map[string]any{"client_id": 1, "emission_date": "2026-10-27"}},
		{name: "mileage report without trips", tool: "create_mileage_report", arguments: map[string]any{"name": "x", "date": "2026-10-27", "travel_ids": []int64{}}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			session := connect(t, NewHandler(&fakeAPI{}, nil, 1, t.TempDir()))

			result := call(t, session, testCase.tool, testCase.arguments)

			assert.True(t, result.IsError)
		})
	}
}
