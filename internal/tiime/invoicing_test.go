package tiime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testVATTypes = []VATType{
	{Code: "normal", VATRates: []VATRate{{Rate: 0.2, StartDate: "2014-01-01"}, {Rate: 0.196, EndDate: "2013-12-31"}}},
	{Code: "intermediate", VATRates: []VATRate{{Rate: 0.1, StartDate: "2014-01-01"}}},
	{Code: "foreigner_exoneration", VATRates: []VATRate{{Rate: 0}}},
}

func TestVATTypeRateAt(
	t *testing.T,
) {
	testCases := []struct {
		name     string
		date     string
		wantRate float64
	}{
		{name: "current rate", date: "2026-10-27", wantRate: 0.2},
		{name: "rate before 2014", date: "2013-06-01", wantRate: 0.196},
		{name: "first day of current rate", date: "2014-01-01", wantRate: 0.2},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			rate, ok := testVATTypes[0].RateAt(testCase.date)

			require.True(t, ok)
			assert.InDelta(t, testCase.wantRate, rate, 0.0001)
		})
	}
}

func TestInvoiceLines(
	t *testing.T,
) {
	testCases := []struct {
		name          string
		lines         []InvoiceLine
		wantExcluding float64
		wantIncluding float64
		wantPerVAT    map[string]map[string]float64
		wantErr       string
	}{
		{
			name:          "single line defaults to normal VAT",
			lines:         []InvoiceLine{{Description: "Development", Quantity: 20, UnitAmount: 750}},
			wantExcluding: 15000,
			wantIncluding: 18000,
			wantPerVAT:    map[string]map[string]float64{"normal": {"total_excluding_taxes": 15000, "vat_amount": 3000}},
		},
		{
			name: "mixed VAT types with rounding",
			lines: []InvoiceLine{
				{Description: "Consulting", Quantity: 1.5, UnitAmount: 333.33, VATType: "normal"},
				{Description: "Meals", Quantity: 1, UnitAmount: 45.55, VATType: "intermediate"},
				{Description: "Mission UK", Quantity: 1, UnitAmount: 1000, VATType: "foreigner_exoneration"},
			},
			wantExcluding: 1545.55,
			wantIncluding: 1650.11,
			wantPerVAT: map[string]map[string]float64{
				"normal":                {"total_excluding_taxes": 500, "vat_amount": 100},
				"intermediate":          {"total_excluding_taxes": 45.55, "vat_amount": 4.56},
				"foreigner_exoneration": {"total_excluding_taxes": 1000, "vat_amount": 0},
			},
		},
		{
			name:    "unknown VAT type",
			lines:   []InvoiceLine{{Description: "X", Quantity: 1, UnitAmount: 1, VATType: "unknown"}},
			wantErr: `unknown VAT type "unknown" on line 1`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lines, totals, err := invoiceLines(testCase.lines, testVATTypes, "2026-10-27")
			if testCase.wantErr != "" {
				require.EqualError(t, err, testCase.wantErr)
				return
			}

			require.NoError(t, err)
			require.Len(t, lines, len(testCase.lines))
			assert.Equal(t, 1, lines[0]["sequence"])
			assert.Equal(t, "benefit", lines[0]["invoicing_category_type"])
			assert.InDelta(t, testCase.wantExcluding, totals["total_excluding_taxes"], 0.001)
			assert.InDelta(t, testCase.wantIncluding, totals["total_including_taxes"], 0.001)
			assert.Equal(t, testCase.wantPerVAT, totals["totals_per_vat_type"])
		})
	}
}

func TestCreateInvoiceDraft(
	t *testing.T,
) {
	var posted map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("GET /invoice/companies/42/invoices", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":7,"status":"paid"}]`))
	})
	mux.HandleFunc("GET /invoice/companies/42/invoices/7", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":7,"status":"paid","compiled_number":"11","template":"advanced",
			"iban":"FR76","payment_condition":"30 days","bank_transactions":[1],"lines":[{"id":1}]}`))
	})
	mux.HandleFunc("GET /invoice/companies/42/clients/9", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":9,"name":"ACME","address":"1 Main Street","postal_code":"69002","city":"Lyon",
			"country_code":"FR","siren_or_siret":null,"intracom_vat_number":"FR123"}`))
	})
	mux.HandleFunc("GET /invoice/invoicing_config/vat_types", func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(testVATTypes))
	})
	mux.HandleFunc("POST /invoice/companies/42/invoices", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&posted))
		_, _ = w.Write([]byte(`{"id":8,"status":"draft"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

	_, err := client.CreateInvoiceDraft(context.Background(), 42, InvoiceDraft{
		ClientID:     9,
		EmissionDate: "2026-10-31",
		Lines:        []InvoiceLine{{Description: "Mission", Quantity: 2, UnitAmount: 500}},
	})

	require.NoError(t, err)
	assert.Equal(t, "draft", posted["status"])
	assert.Equal(t, "advanced", posted["template"])
	assert.Equal(t, "FR76", posted["iban"])
	assert.Equal(t, "ACME", posted["client_name"])
	assert.Equal(t, map[string]any{"id": float64(9)}, posted["client"])
	assert.Equal(t, true, posted["client_intracom_vat_number_enabled"])
	assert.Equal(t, false, posted["client_siren_or_siret_enabled"])
	assert.InDelta(t, 1200.0, posted["total_including_taxes"], 0.001)
	assert.NotContains(t, posted, "compiled_number")
	assert.NotContains(t, posted, "bank_transactions")
}

func TestUpdateInvoiceDraftRejectsSavedInvoices(
	t *testing.T,
) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte(`{"id":7,"status":"paid"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

	_, err := client.UpdateInvoiceDraft(context.Background(), 42, 7, InvoiceDraft{Title: "x"})

	require.EqualError(t, err, "invoice 7 is paid, it can only be changed when draft")
}
