package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcp-tiime/internal/tiime"
)

type labelsAPI struct {
	API
	created  []tiime.Label
	renamed  string
	disabled *bool
}

func (l *labelsAPI) StandardLabels(
	_ context.Context,
	_ int64,
) ([]tiime.Label, error) {
	return []tiime.Label{{Label: "INSURANCE"}, {Label: "GIFTS"}}, nil
}

func (l *labelsAPI) CreateLabel(
	_ context.Context,
	_ int64,
	label tiime.Label,
) (tiime.Label, error) {
	l.created = append(l.created, label)
	label.ID = 99
	return label, nil
}

func (l *labelsAPI) RenameLabel(
	_ context.Context,
	_ int64,
	labelID int64,
	name string,
) (tiime.Label, error) {
	l.renamed = name
	return tiime.Label{ID: labelID, Label: name}, nil
}

func (l *labelsAPI) SetLabelDisabled(
	_ context.Context,
	_ int64,
	labelID int64,
	disabled bool,
) (tiime.Label, error) {
	l.disabled = &disabled
	return tiime.Label{ID: labelID, Disabled: disabled}, nil
}

func (l *labelsAPI) AllTags(
	_ context.Context,
	_ int64,
) ([]tiime.Tag, error) {
	return []tiime.Tag{
		{ID: 1, Name: "Urssaf"},
		{ID: 2, Name: "URSSAF 2022", ArchivedAt: "2023-01-01"},
		{ID: 3, Name: "Qonto"},
	}, nil
}

func TestCreateLabel(
	t *testing.T,
) {
	testCases := []struct {
		name         string
		arguments    map[string]any
		wantError    bool
		wantLabel    string
		wantStandard bool
	}{
		{name: "adopts a standard label case insensitively", arguments: map[string]any{"standard": "insurance"}, wantLabel: "INSURANCE", wantStandard: true},
		{name: "creates a custom label", arguments: map[string]any{"name": "Training"}, wantLabel: "Training"},
		{name: "unknown standard label", arguments: map[string]any{"standard": "unknown"}, wantError: true},
		{name: "requires exactly one of standard and name", arguments: map[string]any{"standard": "GIFTS", "name": "x"}, wantError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			api := &labelsAPI{}
			session := connect(t, NewHandler(api, nil, 1, t.TempDir()))

			result := call(t, session, "create_label", testCase.arguments)

			require.Equal(t, testCase.wantError, result.IsError)
			if testCase.wantError {
				assert.Empty(t, api.created)
				return
			}
			require.Len(t, api.created, 1)
			assert.Equal(t, testCase.wantLabel, api.created[0].Label)
			assert.Equal(t, testCase.wantStandard, api.created[0].IsStandard)
		})
	}
}

func TestUpdateLabel(
	t *testing.T,
) {
	api := &labelsAPI{}
	session := connect(t, NewHandler(api, nil, 1, t.TempDir()))

	require.True(t, call(t, session, "update_label", map[string]any{"id": 5}).IsError)

	result := call(t, session, "update_label", map[string]any{"id": 5, "name": "Software", "disabled": true})

	require.False(t, result.IsError)
	assert.Equal(t, "Software", api.renamed)
	require.NotNil(t, api.disabled)
	assert.True(t, *api.disabled)
}

func TestListTags(
	t *testing.T,
) {
	testCases := []struct {
		name      string
		arguments map[string]any
		wantCount int
	}{
		{name: "active tags only by default", arguments: map[string]any{}, wantCount: 2},
		{name: "search is case insensitive", arguments: map[string]any{"search": "urssaf"}, wantCount: 1},
		{name: "include archived", arguments: map[string]any{"search": "urssaf", "include_archived": true}, wantCount: 2},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			session := connect(t, NewHandler(&labelsAPI{}, nil, 1, t.TempDir()))

			result := call(t, session, "list_tags", testCase.arguments)

			require.False(t, result.IsError)
			output, ok := result.StructuredContent.(map[string]any)
			require.True(t, ok)
			assert.Len(t, output["tags"], testCase.wantCount)
		})
	}
}
