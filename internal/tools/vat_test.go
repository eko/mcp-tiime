package tools

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcp-tiime/internal/tiime"
)

type vatAPI struct {
	API
	year  int
	month int
}

func (v *vatAPI) VATEstimate(
	_ context.Context,
	_ int64,
	year int,
	month int,
) (tiime.VATEstimate, error) {
	v.year, v.month = year, month
	return tiime.VATEstimate{TotalDue: 2957}, nil
}

func TestGetVATEstimate(
	t *testing.T,
) {
	now := time.Now()

	testCases := []struct {
		name      string
		arguments map[string]any
		wantYear  int
		wantMonth int
		wantError bool
	}{
		{name: "explicit month", arguments: map[string]any{"month": "2026-09"}, wantYear: 2026, wantMonth: 9},
		{name: "current month by default", arguments: map[string]any{}, wantYear: now.Year(), wantMonth: int(now.Month())},
		{name: "invalid month", arguments: map[string]any{"month": "09/2026"}, wantError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			api := &vatAPI{}
			session := connect(t, NewHandler(api, nil, 1, t.TempDir()))

			result := call(t, session, "get_vat_estimate", testCase.arguments)

			require.Equal(t, testCase.wantError, result.IsError)
			if !testCase.wantError {
				assert.Equal(t, testCase.wantYear, api.year)
				assert.Equal(t, testCase.wantMonth, api.month)
			}
		})
	}
}
