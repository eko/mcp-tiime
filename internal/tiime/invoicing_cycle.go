package tiime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Mail describes an email sent by Tiime with an invoice, a quote or a
// reminder. Empty fields get the defaults of the web application.
type Mail struct {
	To       []string
	Cc       []string
	Subject  string
	Body     string
	CopyToMe bool
}

// mailKind selects the default subject and body of an email.
type mailKind string

const (
	invoiceMail mailKind = "invoice"
	quoteMail   mailKind = "quotation"
	dunningMail mailKind = "dunning_invoice"
)

// emailQuery sends invoices and reminders by email, not through the
// e-invoicing platform.
var emailQuery = url.Values{"email": {"true"}, "pdp": {"false"}}

// SendInvoice emails an invoice to the client.
func (c *Client) SendInvoice(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
	mail Mail,
) error {
	payload, err := c.mailPayload(ctx, companyID, invoiceDocument, invoiceID, invoiceMail, mail)
	if err != nil {
		return err
	}

	return c.postSales(ctx, salesPath(companyID, invoiceDocument, "/"+strconv.FormatInt(invoiceID, 10)+"/send"), emailQuery, payload, nil)
}

// SendQuote emails a quote to the client.
func (c *Client) SendQuote(
	ctx context.Context,
	companyID int64,
	quoteID int64,
	mail Mail,
) error {
	payload, err := c.mailPayload(ctx, companyID, quoteDocument, quoteID, quoteMail, mail)
	if err != nil {
		return err
	}

	return c.postSales(ctx, salesPath(companyID, quoteDocument, "/"+strconv.FormatInt(quoteID, 10)+"/send"), nil, payload, nil)
}

// SendReminder emails a reminder for an unpaid invoice.
func (c *Client) SendReminder(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
	mail Mail,
) error {
	payload, err := c.mailPayload(ctx, companyID, invoiceDocument, invoiceID, dunningMail, mail)
	if err != nil {
		return err
	}

	body := map[string]any{"invoice": idRef{ID: invoiceID}, "mail": payload}

	return c.postSales(ctx, "/invoice"+companyPath(companyID, "/dunnings"), emailQuery, body, nil)
}

// Reminders lists the reminders sent for an invoice.
func (c *Client) Reminders(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
) (json.RawMessage, error) {
	var raw json.RawMessage
	query := url.Values{"invoice": {strconv.FormatInt(invoiceID, 10)}}
	_, err := c.getJSON(ctx, "/invoice"+companyPath(companyID, "/dunnings"), query, nil, &raw)

	return raw, err
}

// mailPayload builds the email the way the web application does: from the
// user, to the client billing email and contacts, with the default texts.
func (c *Client) mailPayload(
	ctx context.Context,
	companyID int64,
	document salesDocument,
	documentID int64,
	kind mailKind,
	mail Mail,
) (map[string]any, error) {
	detail, err := c.salesDetail(ctx, companyID, document, documentID)
	if err != nil {
		return nil, err
	}

	var user struct {
		Email string `json:"email"`
	}
	if _, err := c.getJSON(ctx, "/users/me", nil, nil, &user); err != nil {
		return nil, fmt.Errorf(
			"loading current user: %w",
			err,
		)
	}

	to := mail.To
	if len(to) == 0 {
		if to, err = c.billingEmails(ctx, companyID, detail); err != nil {
			return nil, err
		}
	}
	if len(to) == 0 {
		return nil, fmt.Errorf("the client has no billing email: set the recipients")
	}

	subject, body := mail.Subject, mail.Body
	if subject == "" || body == "" {
		var translations map[string]any
		if _, err := c.getJSON(ctx, "/invoice"+companyPath(companyID, "/countries/FR/invoicing_translations"), nil, nil, &translations); err != nil {
			return nil, err
		}
		if subject == "" {
			subject, _ = translations[string(kind)+"_mail_subject"].(string)
		}
		if body == "" {
			body, _ = translations[string(kind)+"_mail_body"].(string)
		}
	}

	cci := []string{}
	if mail.CopyToMe {
		cci = append(cci, user.Email)
	}
	cc := mail.Cc
	if cc == nil {
		cc = []string{}
	}

	return map[string]any{
		"from":              user.Email,
		"reply_to":          user.Email,
		"to":                to,
		"cc":                cc,
		"cci":               cci,
		"subject":           subject,
		"body":              body,
		"filename":          detail["pdf_filename"],
		"extra_attachments": []any{},
	}, nil
}

// billingEmails returns the client email and the contacts that receive invoices.
func (c *Client) billingEmails(
	ctx context.Context,
	companyID int64,
	detail map[string]any,
) ([]string, error) {
	clientID, ok := detail["client_id"].(float64)
	if !ok {
		if client, isMap := detail["client"].(map[string]any); isMap {
			clientID, ok = client["id"].(float64)
		}
	}
	if !ok {
		return nil, nil
	}

	var client struct {
		Email             string `json:"email"`
		InvoicingUseEmail bool   `json:"invoicing_use_email"`
	}
	path := "/invoice" + companyPath(companyID, "/clients/"+strconv.FormatInt(int64(clientID), 10))
	if _, err := c.getJSON(ctx, path, nil, nil, &client); err != nil {
		return nil, err
	}

	var emails []string
	if client.InvoicingUseEmail && client.Email != "" {
		emails = append(emails, client.Email)
	}

	var contacts []struct {
		Email             string `json:"email"`
		InvoicingUseEmail bool   `json:"invoicing_use_email"`
	}
	if _, err := c.getJSON(ctx, path+"/contacts", nil, nil, &contacts); err != nil {
		return nil, err
	}
	for _, contact := range contacts {
		if contact.InvoicingUseEmail && contact.Email != "" {
			emails = append(emails, contact.Email)
		}
	}

	return emails, nil
}

// DuplicateInvoice copies an invoice into a new draft.
func (c *Client) DuplicateInvoice(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
) (json.RawMessage, error) {
	return c.postSalesRaw(ctx, salesPath(companyID, invoiceDocument, "/"+strconv.FormatInt(invoiceID, 10)+"/duplicate"), nil)
}

// DuplicateQuote copies a quote into a new one.
func (c *Client) DuplicateQuote(
	ctx context.Context,
	companyID int64,
	quoteID int64,
) (json.RawMessage, error) {
	return c.postSalesRaw(ctx, salesPath(companyID, quoteDocument, "/"+strconv.FormatInt(quoteID, 10)+"/duplicate"), nil)
}

// DeleteInvoiceDraft deletes an invoice that is still a draft.
func (c *Client) DeleteInvoiceDraft(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
) error {
	_, err := c.call(ctx, request{method: http.MethodDelete, path: salesPath(companyID, invoiceDocument, "/"+strconv.FormatInt(invoiceID, 10)+"/draft")}, nil)

	return err
}

// DeleteQuote deletes a quote.
func (c *Client) DeleteQuote(
	ctx context.Context,
	companyID int64,
	quoteID int64,
) error {
	_, err := c.call(ctx, request{method: http.MethodDelete, path: salesPath(companyID, quoteDocument, "/"+strconv.FormatInt(quoteID, 10))}, nil)

	return err
}

// FinalizeInvoice numbers a draft invoice, which can no longer be changed.
func (c *Client) FinalizeInvoice(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
) (json.RawMessage, error) {
	var config struct {
		NumberTemplate string `json:"number_template"`
	}
	if _, err := c.getJSON(ctx, "/invoice"+companyPath(companyID, "/invoicing_config"), nil, nil, &config); err != nil {
		return nil, err
	}

	extra := map[string]any{"status": "saved"}
	if config.NumberTemplate != "" {
		extra["number_template"] = config.NumberTemplate
	}

	return c.updateSalesDocument(ctx, companyID, invoiceDocument, invoiceID, InvoiceDraft{}, extra)
}

// CancelInvoice cancels a numbered invoice with a credit note.
func (c *Client) CancelInvoice(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
	date string,
) (json.RawMessage, error) {
	body := map[string]any{"emission_date": date, "cancel_invoice": true}

	return c.postSalesRaw(ctx, salesPath(companyID, invoiceDocument, "/"+strconv.FormatInt(invoiceID, 10)+"/credit_note"), body)
}

// CreateCreditNote creates a credit note draft for an invoice, to refund part of it.
func (c *Client) CreateCreditNote(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
) (json.RawMessage, error) {
	return c.postSalesRaw(ctx, salesPath(companyID, invoiceDocument, "/"+strconv.FormatInt(invoiceID, 10)+"/credit_note"), nil)
}

// InvoicePayment is a payment recorded manually on an invoice.
type InvoicePayment struct {
	Date   string
	Amount float64
	// Method is bank_transfer (default), card, check, cash or levy.
	Method string
}

// RecordInvoicePayment records a payment received outside a matched bank
// transaction.
func (c *Client) RecordInvoicePayment(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
	payment InvoicePayment,
) (json.RawMessage, error) {
	method := payment.Method
	if method == "" {
		method = "bank_transfer"
	}

	body := map[string]any{
		"invoice_id":     invoiceID,
		"date":           payment.Date,
		"amount":         payment.Amount,
		"operation_type": method,
		"source":         "manual",
		// Stored in Tiime and shown in its French interface: keep the label
		// the web application writes for manual payments.
		"label": "Encaissement manuel",
	}

	var raw json.RawMessage
	err := c.sendJSON(ctx, http.MethodPost, companyPath(companyID, "/collected_payments"), body, &raw)

	return raw, err
}

// InvoicePayments lists the payments collected on an invoice.
func (c *Client) InvoicePayments(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
) (json.RawMessage, error) {
	var raw json.RawMessage
	query := url.Values{"invoice": {strconv.FormatInt(invoiceID, 10)}}
	_, err := c.getJSON(ctx, companyPath(companyID, "/collected_payments"), query, nil, &raw)

	return raw, err
}

// CancelInvoicePayment cancels a payment recorded manually.
func (c *Client) CancelInvoicePayment(
	ctx context.Context,
	companyID int64,
	paymentUUID string,
) error {
	return c.sendJSON(ctx, http.MethodPost, companyPath(companyID, "/collected_payments/"+url.PathEscape(paymentUUID)+"/cancel"), nil, nil)
}

// Article is an item of the invoicing catalogue.
type Article struct {
	ID          int64   `json:"id,omitempty"`
	Code        string  `json:"code" jsonschema:"Article name or code"`
	Description string  `json:"description,omitempty" jsonschema:"Description copied on invoice lines"`
	UnitAmount  float64 `json:"unit_amount" jsonschema:"Unit price excluding taxes"`
	VATType     string  `json:"vat_type,omitempty" jsonschema:"VAT type code, see list_vat_types; defaults to normal"`
	// CategoryType is "benefit" for services or "sale" for goods.
	CategoryType string `json:"invoicing_category_type,omitempty" jsonschema:"benefit for services (default) or sale for goods"`
}

// Articles lists the invoicing catalogue.
func (c *Client) Articles(
	ctx context.Context,
	companyID int64,
	search string,
	window Range,
) (Page[json.RawMessage], error) {
	query := url.Values{}
	if search != "" {
		query.Set("q", search)
	}

	return getPage[json.RawMessage](ctx, c, "/invoice"+companyPath(companyID, "/articles"), query, nil, window, "")
}

// SaveArticle creates an article, or updates it when article.ID is set.
func (c *Client) SaveArticle(
	ctx context.Context,
	companyID int64,
	article Article,
) (json.RawMessage, error) {
	vatType := article.VATType
	if vatType == "" {
		vatType = "normal"
	}
	category := article.CategoryType
	if category == "" {
		category = "benefit"
	}

	body := map[string]any{
		"code":                    article.Code,
		"description":             article.Description,
		"unit_amount":             article.UnitAmount,
		"vat_type":                map[string]string{"code": vatType},
		"invoicing_unit":          nil,
		"invoicing_category_type": category,
	}

	path := "/invoice" + companyPath(companyID, "/articles")
	method := http.MethodPost
	if article.ID != 0 {
		path += "/" + strconv.FormatInt(article.ID, 10)
		method = http.MethodPatch
	}

	var raw json.RawMessage
	err := c.sendJSON(ctx, method, path, body, &raw)

	return raw, err
}

// DeleteArticle deletes an article of the catalogue.
func (c *Client) DeleteArticle(
	ctx context.Context,
	companyID int64,
	articleID int64,
) error {
	_, err := c.call(ctx, request{method: http.MethodDelete, path: "/invoice" + companyPath(companyID, "/articles/"+strconv.FormatInt(articleID, 10))}, nil)

	return err
}

// InvoicingUnit is a unit usable on invoice lines.
type InvoicingUnit struct {
	ID       int64  `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Favorite bool   `json:"favorite,omitempty"`
}

// InvoicingUnits lists the units usable on invoice lines, e.g. day or hour,
// with their French name.
func (c *Client) InvoicingUnits(
	ctx context.Context,
	companyID int64,
) ([]InvoicingUnit, error) {
	var units []struct {
		InvoicingUnit
		Translations []struct {
			Lang     string `json:"lang"`
			Singular string `json:"singular"`
		} `json:"translations"`
	}
	_, err := c.getJSON(ctx, "/invoice"+companyPath(companyID, "/invoicing_units"), nil, map[string]string{"Range": "items=0-199"}, &units)
	if err != nil {
		return nil, err
	}

	result := make([]InvoicingUnit, 0, len(units))
	for _, unit := range units {
		for _, translation := range unit.Translations {
			if translation.Lang == "FR" {
				unit.Name = translation.Singular
			}
		}
		result = append(result, unit.InvoicingUnit)
	}

	return result, nil
}

func (c *Client) postSales(
	ctx context.Context,
	path string,
	query url.Values,
	payload any,
	out any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = c.call(ctx, request{method: http.MethodPost, path: path, query: query, body: body, contentType: "application/json"}, out)

	return err
}

func (c *Client) postSalesRaw(
	ctx context.Context,
	path string,
	payload any,
) (json.RawMessage, error) {
	var raw json.RawMessage
	err := c.postSales(ctx, path, nil, payload, &raw)

	return raw, err
}
