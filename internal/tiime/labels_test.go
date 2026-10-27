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

func TestUpdateTag(
	t *testing.T,
) {
	archived := true
	restored := false

	testCases := []struct {
		name         string
		rename       string
		archived     *bool
		wantName     string
		wantArchived bool
	}{
		{name: "rename keeps the other fields", rename: "URSSAF", wantName: "URSSAF"},
		{name: "archive", archived: &archived, wantName: "Urssaf", wantArchived: true},
		{name: "restore", archived: &restored, wantName: "Urssaf"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var patched map[string]any
			mux := http.NewServeMux()
			mux.HandleFunc("GET /accounts/companies/42/tags", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`[{"id":1,"name":"Other"},{"id":7,"name":"Urssaf","type":"accountant","archived_at":"2026-01-01T00:00:00Z"}]`))
			})
			mux.HandleFunc("PATCH /accounts/companies/42/tags/7", func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewDecoder(r.Body).Decode(&patched))
				_, _ = w.Write([]byte(`{"id":7}`))
			})
			server := httptest.NewServer(mux)
			defer server.Close()

			client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

			_, err := client.UpdateTag(context.Background(), 42, 7, testCase.rename, testCase.archived)

			require.NoError(t, err)
			assert.Equal(t, testCase.wantName, patched["name"])
			assert.Equal(t, "accountant", patched["type"])
			assert.Contains(t, patched, "isAutomatic")
			if testCase.archived == nil {
				assert.Equal(t, "2026-01-01T00:00:00Z", patched["archivedAt"])
			} else {
				assert.Equal(t, testCase.wantArchived, patched["archivedAt"] != nil)
			}
		})
	}
}

func TestUpdateTagNotFound(
	t *testing.T,
) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

	_, err := client.UpdateTag(context.Background(), 42, 7, "x", nil)

	require.ErrorIs(t, err, ErrNotFoundInList)
}

func TestSearchLabelsPaginates(
	t *testing.T,
) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		assert.Equal(t, labelsAccept, r.Header.Get("Accept"))
		assert.Equal(t, "fees", r.URL.Query().Get("q"))
		if r.Header.Get("Range") == "items=0-99" {
			w.Header().Set("Content-Range", "items 0-99/*")
			labels := make([]Label, 100)
			require.NoError(t, json.NewEncoder(w).Encode(labels))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "1", &staticTokens{tokens: []string{"t"}}, server.Client())

	labels, err := client.SearchLabels(context.Background(), 42, "FEES")

	require.NoError(t, err)
	assert.Len(t, labels, 100)
	assert.Equal(t, 2, calls)
}
