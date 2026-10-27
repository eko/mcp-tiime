package tiime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// BankAccounts lists the bank accounts synchronized with Tiime.
func (c *Client) BankAccounts(
	ctx context.Context,
	companyID int64,
) ([]BankAccount, error) {
	var accounts []BankAccount
	_, err := c.getJSON(ctx, accountsPath(companyID, "/bank_accounts"), nil, nil, &accounts)

	return accounts, err
}

// BankTransactions lists bank transactions, most recent first.
func (c *Client) BankTransactions(
	ctx context.Context,
	companyID int64,
	filter TransactionFilter,
	window Range,
) (Page[BankTransaction], error) {
	query := url.Values{}
	if filter.Search != "" {
		query.Set("q", filter.Search)
	}
	if filter.BankAccountID != 0 {
		query.Set("bank_account", strconv.FormatInt(filter.BankAccountID, 10))
	}

	if filter.Date != "" {
		query.Set("date", filter.Date)
	}
	if filter.Matched != nil {
		query.Set("matched", strconv.FormatBool(*filter.Matched))
	}

	// The v2 representation carries the matched documents and invoices counts.
	headers := map[string]string{"Accept": "application/vnd.tiime.bank_transactions.v2+json"}

	return getPage[BankTransaction](ctx, c, accountsPath(companyID, "/bank_transactions"), query, headers, window, "transactions")
}

// BankTransaction returns the full detail of a bank transaction, including its
// imputations and matched documents.
func (c *Client) BankTransaction(
	ctx context.Context,
	companyID int64,
	transactionID int64,
) (json.RawMessage, error) {
	headers := map[string]string{"Accept": "application/vnd.tiime.docs.imputation+json"}

	return c.getRaw(ctx, transactionPath(companyID, transactionID, ""), headers)
}

// TransactionMatchings lists the documents and invoices matched with a transaction.
func (c *Client) TransactionMatchings(
	ctx context.Context,
	companyID int64,
	transactionID int64,
) ([]Matching, error) {
	var body struct {
		Matchings []Matching `json:"matchings"`
	}
	_, err := c.getJSON(ctx, transactionPath(companyID, transactionID, "/matchings"), nil, nil, &body)

	return body.Matchings, err
}

// SetTransactionDocuments replaces the documents matched with a transaction.
// Invoices are matched through a separate endpoint and are left untouched.
func (c *Client) SetTransactionDocuments(
	ctx context.Context,
	companyID int64,
	transactionID int64,
	documentIDs []int64,
) error {
	payload := struct {
		Documents []idRef `json:"documents"`
	}{Documents: idRefs(documentIDs)}

	return c.sendJSON(ctx, http.MethodPut, transactionPath(companyID, transactionID, "/document_matchings"), payload, nil)
}

// SetTransactionInvoices replaces the invoices matched with a transaction.
// Documents are matched through a separate endpoint and are left untouched.
func (c *Client) SetTransactionInvoices(
	ctx context.Context,
	companyID int64,
	transactionID int64,
	invoiceIDs []int64,
) error {
	return c.sendJSON(ctx, http.MethodPut, transactionPath(companyID, transactionID, "/invoice_matchings"), idRefs(invoiceIDs), nil)
}

// LabelSuggestions returns the accounting labels Tiime suggests for a transaction.
func (c *Client) LabelSuggestions(
	ctx context.Context,
	companyID int64,
	transactionID int64,
) ([]Label, error) {
	var labels []Label
	headers := map[string]string{"Accept": "application/vnd.tiime.bank_transactions.label_suggestions.v2+json"}
	_, err := c.getJSON(ctx, transactionPath(companyID, transactionID, "/label_suggestions"), nil, headers, &labels)

	return labels, err
}

// AllBankTransactions walks the pages of BankTransactions and returns at
// most limit transactions.
func (c *Client) AllBankTransactions(
	ctx context.Context,
	companyID int64,
	filter TransactionFilter,
	limit int,
) ([]BankTransaction, bool, error) {
	var transactions []BankTransaction
	for len(transactions) < limit {
		page, err := c.BankTransactions(ctx, companyID, filter, Range{Offset: len(transactions), Limit: min(100, limit-len(transactions))})
		if err != nil {
			return nil, false, err
		}
		transactions = append(transactions, page.Items...)
		if !page.HasMore || len(page.Items) == 0 {
			return transactions, false, nil
		}
	}

	return transactions, true, nil
}

// AnnotateTransactions updates the comment, accounting label or tags of
// several transactions at once. Each annotation is sent as a separate call,
// as the web application does.
func (c *Client) AnnotateTransactions(
	ctx context.Context,
	companyID int64,
	transactionIDs []int64,
	annotation TransactionAnnotation,
) error {
	var updates []map[string]any
	if annotation.Comment != nil {
		// Tiime ignores an empty comment: null clears it.
		updates = append(updates, map[string]any{"comment": nilIfEmpty(*annotation.Comment)})
	}
	if annotation.LabelID != 0 {
		updates = append(updates, map[string]any{
			"imputations": []map[string]any{{"label": idRef{ID: annotation.LabelID}}},
		})
	}
	if annotation.Tags != nil {
		tags := make([]map[string]string, 0, len(annotation.Tags))
		for _, name := range annotation.Tags {
			tags = append(tags, map[string]string{"name": name})
		}
		updates = append(updates, map[string]any{"tags": tags})
	}

	for _, update := range updates {
		payload := map[string]any{
			"bank_transaction_ids": transactionIDs,
			"bank_transaction":     update,
		}
		if err := c.sendJSON(ctx, http.MethodPatch, accountsPath(companyID, "/bank_transactions"), payload, nil); err != nil {
			return err
		}
	}

	return nil
}

type idRef struct {
	ID int64 `json:"id"`
}

func idRefs(
	ids []int64,
) []idRef {
	refs := make([]idRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, idRef{ID: id})
	}

	return refs
}

func accountsPath(
	companyID int64,
	suffix string,
) string {
	return "/accounts" + companyPath(companyID, suffix)
}

func transactionPath(
	companyID int64,
	transactionID int64,
	suffix string,
) string {
	return accountsPath(companyID, "/bank_transactions/"+strconv.FormatInt(transactionID, 10)+suffix)
}
