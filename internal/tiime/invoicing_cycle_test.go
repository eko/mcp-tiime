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

func invoicingServer(
	t *testing.T,
	posted *map[string]any,
	query *string,
) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /invoice/companies/42/invoices/7", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":7,"status":"draft","client_id":9,"pdf_filename":"invoice-11.pdf","lines":[]}`))
	})
	mux.HandleFunc("GET /users/me", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":1,"email":"me@example.com"}`))
	})
	mux.HandleFunc("GET /invoice/companies/42/clients/9", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":9,"email":"billing@acme.test","invoicing_use_email":true}`))
	})
	mux.HandleFunc("GET /invoice/companies/42/clients/9/contacts", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"email":"cfo@acme.test","invoicing_use_email":true},{"email":"dev@acme.test","invoicing_use_email":false}]`))
	})
	mux.HandleFunc("GET /invoice/companies/42/countries/FR/invoicing_translations", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"invoice_mail_subject":"Your invoice","invoice_mail_body":"Dear customer","dunning_invoice_mail_subject":"Reminder"}`))
	})
	mux.HandleFunc("GET /invoice/companies/42/invoicing_config", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"number_template":"X"}`))
	})
	record := func(w http.ResponseWriter, r *http.Request) {
		*query = r.URL.RawQuery
		require.NoError(t, json.NewDecoder(r.Body).Decode(posted))
		_, _ = w.Write([]byte(`{"id":7}`))
	}
	mux.HandleFunc("POST /invoice/companies/42/invoices/7/send", record)
	mux.HandleFunc("POST /invoice/companies/42/dunnings", record)
	mux.HandleFunc("PATCH /invoice/companies/42/invoices/7", record)

	return httptest.NewServer(mux)
}

func TestSendInvoice(
	t *testing.T,
) {
	testCases := []struct {
		name        string
		mail        Mail
		wantTo      []any
		wantSubject string
		wantCci     []any
	}{
		{
			name:        "defaults to billing emails and template",
			wantTo:      []any{"billing@acme.test", "cfo@acme.test"},
			wantSubject: "Your invoice",
			wantCci:     []any{},
		},
		{
			name:        "explicit recipients, subject and copy",
			mail:        Mail{To: []string{"other@acme.test"}, Subject: "October invoice", CopyToMe: true},
			wantTo:      []any{"other@acme.test"},
			wantSubject: "October invoice",
			wantCci:     []any{"me@example.com"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var posted map[string]any
			var query string
			server := invoicingServer(t, &posted, &query)
			defer server.Close()

			client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

			require.NoError(t, client.SendInvoice(context.Background(), 42, 7, testCase.mail))

			assert.Equal(t, "email=true&pdp=false", query)
			assert.Equal(t, testCase.wantTo, posted["to"])
			assert.Equal(t, testCase.wantSubject, posted["subject"])
			assert.Equal(t, "Dear customer", posted["body"])
			assert.Equal(t, testCase.wantCci, posted["cci"])
			assert.Equal(t, "me@example.com", posted["from"])
			assert.Equal(t, "invoice-11.pdf", posted["filename"])
		})
	}
}

func TestSendReminder(
	t *testing.T,
) {
	var posted map[string]any
	var query string
	server := invoicingServer(t, &posted, &query)
	defer server.Close()

	client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

	require.NoError(t, client.SendReminder(context.Background(), 42, 7, Mail{}))

	assert.Equal(t, map[string]any{"id": float64(7)}, posted["invoice"])
	mail, ok := posted["mail"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Reminder", mail["subject"])
}

func TestFinalizeInvoice(
	t *testing.T,
) {
	var posted map[string]any
	var query string
	server := invoicingServer(t, &posted, &query)
	defer server.Close()

	client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

	_, err := client.FinalizeInvoice(context.Background(), 42, 7)

	require.NoError(t, err)
	assert.Equal(t, "saved", posted["status"])
	assert.Equal(t, "X", posted["number_template"])
	assert.NotContains(t, posted, "pdf_filename")
}

func TestForecastEnabled(
	t *testing.T,
) {
	testCases := []struct {
		name    string
		status  int
		body    string
		enabled bool
	}{
		{name: "not activated", status: http.StatusNoContent},
		{name: "activated", status: http.StatusOK, body: `{"activated_at":"2026-10-27 10:00:00"}`, enabled: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(testCase.status)
				_, _ = w.Write([]byte(testCase.body))
			}))
			defer server.Close()

			client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

			enabled, err := client.ForecastEnabled(context.Background(), 42)

			require.NoError(t, err)
			assert.Equal(t, testCase.enabled, enabled)
		})
	}
}
