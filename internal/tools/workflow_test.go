package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcp-tiime/internal/tiime"
)

type fakeAuth struct {
	challenges []*tiime.LoginChallenge
	codes      []string
}

func (f *fakeAuth) StartLogin(
	_ context.Context,
	_ string,
) (*tiime.LoginChallenge, error) {
	return f.next(), nil
}

func (f *fakeAuth) CompleteLogin(
	_ context.Context,
	code string,
) (*tiime.LoginChallenge, error) {
	f.codes = append(f.codes, code)
	return f.next(), nil
}

func (f *fakeAuth) next() *tiime.LoginChallenge {
	challenge := f.challenges[0]
	f.challenges = f.challenges[1:]
	return challenge
}

type workflowAPI struct {
	API
	matchings  []tiime.Matching
	setDocs    [][]int64
	categories []tiime.DocumentCategory
	uploadedTo int64
	updates    []tiime.DocumentUpdate
}

func (w *workflowAPI) TransactionMatchings(
	_ context.Context,
	_ int64,
	_ int64,
) ([]tiime.Matching, error) {
	return w.matchings, nil
}

func (w *workflowAPI) SetTransactionDocuments(
	_ context.Context,
	_ int64,
	_ int64,
	documentIDs []int64,
) error {
	w.setDocs = append(w.setDocs, documentIDs)
	return nil
}

func (w *workflowAPI) DocumentCategories(
	_ context.Context,
	_ int64,
) ([]tiime.DocumentCategory, error) {
	return w.categories, nil
}

func (w *workflowAPI) UploadDocument(
	_ context.Context,
	_ int64,
	categoryID int64,
	filename string,
	_ []byte,
) (tiime.AccountingDocument, error) {
	w.uploadedTo = categoryID
	return tiime.AccountingDocument{ID: 42, Name: filename}, nil
}

func (w *workflowAPI) UpdateDocument(
	_ context.Context,
	_ int64,
	documentID int64,
	update tiime.DocumentUpdate,
) (tiime.AccountingDocument, error) {
	w.updates = append(w.updates, update)
	return tiime.AccountingDocument{ID: documentID}, nil
}

func call(
	t *testing.T,
	session *mcp.ClientSession,
	tool string,
	arguments map[string]any,
) *mcp.CallToolResult {
	t.Helper()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: arguments})
	require.NoError(t, err)

	return result
}

func TestLoginTools(
	t *testing.T,
) {
	sentAt := time.Date(2026, 10, 26, 21, 0, 0, 0, time.UTC)
	auth := &fakeAuth{challenges: []*tiime.LoginChallenge{
		{Method: "email", Message: "Code sent to v***@gmail.com", SentAt: sentAt},
		{Method: "email", Message: "Invalid code", SentAt: sentAt},
		nil,
	}}
	session := connect(t, NewHandler(&fakeAPI{}, auth, 1, t.TempDir()))

	testCases := []struct {
		name       string
		tool       string
		arguments  map[string]any
		wantStatus string
	}{
		{
			name:       "start asks for a code",
			tool:       "start_login",
			arguments:  map[string]any{},
			wantStatus: "code_required",
		},
		{
			name:       "wrong code asks again",
			tool:       "complete_login",
			arguments:  map[string]any{"code": "000000"},
			wantStatus: "code_required",
		},
		{
			name:       "right code logs in",
			tool:       "complete_login",
			arguments:  map[string]any{"code": "123456"},
			wantStatus: "logged_in",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := call(t, session, testCase.tool, testCase.arguments)

			require.False(t, result.IsError)
			output, ok := result.StructuredContent.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, testCase.wantStatus, output["status"])
			if testCase.wantStatus == "code_required" {
				assert.Equal(t, "2026-10-26T21:00:00Z", output["sent_at"])
			}
		})
	}

	assert.Equal(t, []string{"000000", "123456"}, auth.codes)
}

func TestTransactionDocuments(
	t *testing.T,
) {
	testCases := []struct {
		name     string
		tool     string
		docs     []int64
		wantDocs []int64
	}{
		{
			name:     "link keeps existing documents and ignores invoices",
			tool:     "link_documents_to_transaction",
			docs:     []int64{2, 1},
			wantDocs: []int64{1, 2},
		},
		{
			name:     "unlink removes only the given documents",
			tool:     "unlink_documents_from_transaction",
			docs:     []int64{1},
			wantDocs: []int64{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			api := &workflowAPI{matchings: []tiime.Matching{{ID: 1, Type: "receipt"}, {ID: 9, Type: "invoice"}}}
			session := connect(t, NewHandler(api, nil, 1, t.TempDir()))

			result := call(t, session, testCase.tool, map[string]any{"transaction_id": 5, "document_ids": testCase.docs})

			require.False(t, result.IsError)
			require.Len(t, api.setDocs, 1)
			assert.ElementsMatch(t, testCase.wantDocs, api.setDocs[0])
		})
	}
}

func TestUploadDocument(
	t *testing.T,
) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ticket.pdf")
	require.NoError(t, os.WriteFile(file, []byte("%PDF-1.7"), 0o600))

	testCases := []struct {
		name         string
		arguments    map[string]any
		wantError    bool
		wantCategory int64
		wantUpdates  int
	}{
		{
			name:         "defaults to receipts without metadata",
			arguments:    map[string]any{"file_path": file},
			wantCategory: 10,
		},
		{
			name:         "advanced expense with metadata",
			arguments:    map[string]any{"file_path": file, "category": "advanced_expense", "date": "2026-10-01", "amount": 12.5},
			wantCategory: 20,
			wantUpdates:  1,
		},
		{
			name:      "rejects relative paths",
			arguments: map[string]any{"file_path": "ticket.pdf"},
			wantError: true,
		},
		{
			name:      "rejects unknown categories",
			arguments: map[string]any{"file_path": file, "category": "unknown"},
			wantError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			api := &workflowAPI{categories: []tiime.DocumentCategory{
				{ID: 10, Identifier: "receipt"},
				{ID: 20, Identifier: "advanced_expense"},
			}}
			session := connect(t, NewHandler(api, nil, 1, t.TempDir()))

			result := call(t, session, "upload_document", testCase.arguments)

			assert.Equal(t, testCase.wantError, result.IsError)
			assert.Equal(t, testCase.wantCategory, api.uploadedTo)
			assert.Len(t, api.updates, testCase.wantUpdates)
		})
	}
}

func TestCreateExpenseReportRequiresExpenses(
	t *testing.T,
) {
	session := connect(t, NewHandler(&workflowAPI{}, nil, 1, t.TempDir()))

	result := call(t, session, "create_expense_report", map[string]any{
		"name":         "Expenses November 2026",
		"date":         "2026-10-27",
		"document_ids": []int64{},
	})

	require.True(t, result.IsError)
	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, "at least one expense")
}
