package tiime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticTokens struct {
	tokens      []string
	invalidated int
}

func (s *staticTokens) AccessToken(
	_ context.Context,
) (string, error) {
	return s.tokens[min(s.invalidated, len(s.tokens)-1)], nil
}

func (s *staticTokens) Invalidate() {
	s.invalidated++
}

func TestParseContentRange(
	t *testing.T,
) {
	total := 120

	testCases := []struct {
		name        string
		header      string
		fullPage    bool
		wantHasMore bool
		wantTotal   *int
	}{
		{
			name:        "known total with more items",
			header:      "items 0-49/120",
			wantHasMore: true,
			wantTotal:   &total,
		},
		{
			name:        "known total on last page",
			header:      "items 100-119/120",
			wantHasMore: false,
			wantTotal:   &total,
		},
		{
			name:        "unknown total",
			header:      "items 0-49/*",
			wantHasMore: true,
		},
		{
			name:        "missing header with full page",
			header:      "",
			fullPage:    true,
			wantHasMore: true,
		},
		{
			name:        "missing header with partial page",
			header:      "",
			wantHasMore: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			hasMore, gotTotal := parseContentRange(testCase.header, testCase.fullPage)

			assert.Equal(t, testCase.wantHasMore, hasMore)
			assert.Equal(t, testCase.wantTotal, gotTotal)
		})
	}
}

func TestClientInvoices(
	t *testing.T,
) {
	testCases := []struct {
		name      string
		filter    DocumentFilter
		window    Range
		wantRange string
		wantQuery string
	}{
		{
			name:      "default window",
			wantRange: "items=0-49",
			wantQuery: "",
		},
		{
			name:      "filters and offset",
			filter:    DocumentFilter{Status: "paid", EmissionDate: ">=2026-01-01"},
			window:    Range{Offset: 20, Limit: 10},
			wantRange: "items=20-29",
			wantQuery: "emission_date=%3E%3D2026-01-01&status=paid",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/companies/42/invoices", r.URL.Path)
				assert.Equal(t, testCase.wantQuery, r.URL.RawQuery)
				assert.Equal(t, testCase.wantRange, r.Header.Get("Range"))
				assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
				assert.Equal(t, "tiime", r.Header.Get("tiime-app"))
				assert.Equal(t, "1.2.3", r.Header.Get("tiime-app-version"))

				w.Header().Set("Content-Range", "items 0-0/1")
				_, _ = w.Write([]byte(`[{"id":7,"compiled_number":"F-7","status":"paid","total_including_taxes":120.5}]`))
			}))
			defer server.Close()

			client := NewClient(server.URL, "1.2.3", &staticTokens{tokens: []string{"token"}}, server.Client())

			page, err := client.Invoices(context.Background(), 42, testCase.filter, testCase.window)

			require.NoError(t, err)
			require.Len(t, page.Items, 1)
			assert.Equal(t, "F-7", page.Items[0].CompiledNumber)
			assert.InDelta(t, 120.5, page.Items[0].TotalIncludingTaxes, 0.001)
			assert.False(t, page.HasMore)
		})
	}
}

func TestClientErrors(
	t *testing.T,
) {
	testCases := []struct {
		name            string
		statuses        []int
		wantErr         error
		wantAPIStatus   int
		wantInvalidated int
	}{
		{
			name:            "retries once on unauthorized",
			statuses:        []int{http.StatusUnauthorized, http.StatusOK},
			wantInvalidated: 1,
		},
		{
			name:            "gives up after second unauthorized",
			statuses:        []int{http.StatusUnauthorized, http.StatusUnauthorized},
			wantAPIStatus:   http.StatusUnauthorized,
			wantInvalidated: 1,
		},
		{
			name:     "maps not found",
			statuses: []int{http.StatusNotFound},
			wantErr:  ErrNotFound,
		},
		{
			name:          "returns API errors",
			statuses:      []int{http.StatusInternalServerError},
			wantAPIStatus: http.StatusInternalServerError,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(testCase.statuses[calls])
				calls++
				_, _ = w.Write([]byte(`{"id":1}`))
			}))
			defer server.Close()

			tokens := &staticTokens{tokens: []string{"expired", "fresh"}}
			client := NewClient(server.URL, "1.2.3", tokens, server.Client())

			raw, err := client.Invoice(context.Background(), 42, 1)

			assert.Equal(t, testCase.wantInvalidated, tokens.invalidated)
			switch {
			case testCase.wantErr != nil:
				require.ErrorIs(t, err, testCase.wantErr)
			case testCase.wantAPIStatus != 0:
				var apiErr *APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, testCase.wantAPIStatus, apiErr.StatusCode)
			default:
				require.NoError(t, err)
				assert.JSONEq(t, `{"id":1}`, string(raw))
			}
		})
	}
}
