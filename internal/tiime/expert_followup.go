package tiime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Follow-up of the firm across its portfolio: VAT and annual accounts
// plannings, client indicators, messaging and notifications.

// indicatorsExpand fills every indicator, as the expert application does.
const indicatorsExpand = "company_threads_informations,customer_access_user_on_company,customer_access_user_with_access," +
	"customer_access_user_with_signed_terms,customer_exchanges_not_processed,customer_exchanges_waiting_response," +
	"document_requests,customer_items_to_complete,bank_account_states_total,bank_account_states_warning," +
	"temporary_accounting_entries,tiime_business_status,pdp_config"

// VATPlanning lists the VAT declarations to handle across the portfolio.
func (c *Client) VATPlanning(
	ctx context.Context,
	query ExpertQuery,
	window Range,
) (RawPage, error) {
	values := query.values()
	values.Set("expand", "accounting_config")
	headers := map[string]string{"X-Total-Not-Configured-Company": "true"}

	return c.expertList(ctx, "/expert/users/me/vat_declarations", values, headers, window)
}

// CompanyVATDeclarations lists the VAT declarations of a client company,
// optionally the one ending on endDate.
func (c *Client) CompanyVATDeclarations(
	ctx context.Context,
	companyID int64,
	endDate string,
) (json.RawMessage, error) {
	return c.expertGet(ctx, expertCompanyPath(companyID, "/vat_declarations"), ExpertQuery{Filters: map[string]string{"vat_declaration_end_date": endDate}}.values())
}

// VATDeclaration returns a VAT declaration with its form content.
func (c *Client) VATDeclaration(
	ctx context.Context,
	companyID int64,
	declarationID int64,
) (json.RawMessage, error) {
	return c.expertGet(ctx, expertCompanyPath(companyID, "/vat_declarations"+idPath(declarationID)), nil)
}

// VATConsistencyChecks returns the consistency checks of a VAT declaration.
func (c *Client) VATConsistencyChecks(
	ctx context.Context,
	companyID int64,
	declarationID int64,
) (json.RawMessage, error) {
	headers := map[string]string{"Accept": "application/vnd.tiime.consistency_checks.v4+json"}

	return c.expertGetWith(ctx, expertCompanyPath(companyID, "/vat_declarations"+idPath(declarationID)+"/consistency_check"), nil, headers)
}

// VATDeclarationFile downloads a document of a VAT declaration, such as
// vat_declaration or acknowledgment_receipt.
func (c *Client) VATDeclarationFile(
	ctx context.Context,
	companyID int64,
	declarationID int64,
	docType string,
) ([]byte, string, error) {
	return c.expertDownload(ctx, expertCompanyPath(companyID, "/vat_declarations"+idPath(declarationID)+"/pdf"), map[string]string{"type": docType})
}

// BilanPlanning lists the annual accounts to produce across the portfolio.
func (c *Client) BilanPlanning(
	ctx context.Context,
	query ExpertQuery,
	window Range,
) (RawPage, error) {
	return c.expertList(ctx, "/expert/users/me/bilans", query.values(), nil, window)
}

// Bilan returns the annual accounts of a client company with their
// consistency checks.
func (c *Client) Bilan(
	ctx context.Context,
	companyID int64,
	bilanID int64,
) (json.RawMessage, error) {
	query := url.Values{"expand": {"consistency_checks,metadatas"}}

	return c.expertGet(ctx, expertCompanyPath(companyID, "/bilans"+idPath(bilanID)), query)
}

// ClientIndicators lists, for each client company, what the client still has
// to do: missing receipts, unanswered messages, bank connections…
func (c *Client) ClientIndicators(
	ctx context.Context,
	query ExpertQuery,
	window Range,
) (RawPage, error) {
	values := query.values()
	values.Set("expand", indicatorsExpand)

	return c.expertList(ctx, "/expert/company_accounting_indicators", values, nil, window)
}

// RefreshClientIndicators recomputes the indicators of a client company.
func (c *Client) RefreshClientIndicators(
	ctx context.Context,
	companyID int64,
) (json.RawMessage, error) {
	return c.expertSend(ctx, http.MethodPost, "/expert/company_accounting_indicators/update", nil, map[string]any{"company_ids": []int64{companyID}})
}

// ItemsToRelaunch lists the bank transactions with a pending receipt request
// of an indicator row.
func (c *Client) ItemsToRelaunch(
	ctx context.Context,
	indicatorID int64,
	window Range,
) (RawPage, error) {
	return c.expertList(ctx, "/expert/company_accounting_indicators"+idPath(indicatorID)+"/items_to_relaunch/bank_transactions", nil, nil, window)
}

// ExpertThreads lists the discussion threads with a client company.
func (c *Client) ExpertThreads(
	ctx context.Context,
	companyID int64,
	query ExpertQuery,
	window Range,
) (RawPage, error) {
	return c.expertList(ctx, expertCompanyPath(companyID, "/threads"), query.values(), nil, window)
}

// CreateExpertThread opens a discussion thread with a client company.
func (c *Client) CreateExpertThread(
	ctx context.Context,
	companyID int64,
	thread map[string]any,
) (json.RawMessage, error) {
	return c.expertSend(ctx, http.MethodPost, expertCompanyPath(companyID, "/threads"), nil, thread)
}

// PostExpertThreadMessage posts a message in a discussion thread.
func (c *Client) PostExpertThreadMessage(
	ctx context.Context,
	companyID int64,
	threadID int64,
	content string,
) (json.RawMessage, error) {
	return c.expertSend(ctx, http.MethodPost, expertCompanyPath(companyID, "/threads"+idPath(threadID)+"/messages"), nil, map[string]string{"content": content})
}

// ExpertThreadAction runs an action without body on a thread: read, close,
// unclose, archive or unarchive.
func (c *Client) ExpertThreadAction(
	ctx context.Context,
	companyID int64,
	threadID int64,
	action string,
) error {
	_, err := c.expertSend(ctx, http.MethodPost, expertCompanyPath(companyID, "/threads"+idPath(threadID)+"/"+url.PathEscape(action)), nil, nil)

	return err
}

// Notifications lists the notifications of the firm user, optionally only the
// read or unread ones ("true" or "false").
func (c *Client) Notifications(
	ctx context.Context,
	read string,
	window Range,
) (RawPage, error) {
	return c.expertList(ctx, "/expert/users/me/notifications", ExpertQuery{Filters: map[string]string{"read": read}}.values(), nil, window)
}

// NotificationCount returns the number of unread notifications.
func (c *Client) NotificationCount(
	ctx context.Context,
) (json.RawMessage, error) {
	return c.expertGet(ctx, "/expert/users/me/notifications/count", nil)
}

// MarkNotificationRead marks a notification as read.
func (c *Client) MarkNotificationRead(
	ctx context.Context,
	notificationID int64,
) error {
	_, err := c.expertSend(ctx, http.MethodPatch, "/expert/users/me/notifications"+idPath(notificationID), nil, map[string]bool{"read": true})

	return err
}

// MarkAllNotificationsRead marks every notification as read.
func (c *Client) MarkAllNotificationsRead(
	ctx context.Context,
) error {
	_, err := c.expertSend(ctx, http.MethodPost, "/expert/users/me/notifications/mark_all_as_read", nil, map[string]any{})

	return err
}
