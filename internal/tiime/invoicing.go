package tiime

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// VATType is a VAT regime usable on invoice lines, e.g. "normal" (20%).
type VATType struct {
	Code     string    `json:"code"`
	Name     string    `json:"name,omitempty"`
	VATRates []VATRate `json:"vat_rates,omitempty"`
}

// VATRate is the rate of a VAT type over a period. Rate is a fraction, e.g. 0.2.
type VATRate struct {
	Rate      float64 `json:"rate"`
	StartDate string  `json:"start_date,omitempty"`
	EndDate   string  `json:"end_date,omitempty"`
}

// RateAt returns the rate applicable at date, formatted as YYYY-MM-DD.
func (v VATType) RateAt(
	date string,
) (float64, bool) {
	for _, rate := range v.VATRates {
		if (rate.StartDate == "" || rate.StartDate <= date) && (rate.EndDate == "" || date <= rate.EndDate) {
			return rate.Rate, true
		}
	}

	return 0, false
}

// InvoiceLine is a line of an invoice draft.
type InvoiceLine struct {
	Description string  `json:"description" jsonschema:"Description of the line"`
	Quantity    float64 `json:"quantity" jsonschema:"Quantity"`
	UnitAmount  float64 `json:"unit_amount" jsonschema:"Unit price excluding taxes"`
	// VATType is a VAT type code, see VATTypes.
	VATType string `json:"vat_type,omitempty" jsonschema:"VAT type code, e.g. normal (20%), intermediate (10%), reduced (5.5%), foreigner_exoneration; defaults to normal"`
	// CategoryType is "benefit" for services or "sale" for goods.
	CategoryType string `json:"category_type,omitempty" jsonschema:"benefit for services (default) or sale for goods"`
	UnitID       int64  `json:"unit_id,omitempty" jsonschema:"Unit of the quantity, e.g. day, see list_invoicing_units"`
}

// InvoiceDraft describes an invoice draft. Empty fields are left unchanged on
// updates.
type InvoiceDraft struct {
	ClientID     int64
	EmissionDate string
	DueDate      string
	Title        string
	Comment      string
	Lines        []InvoiceLine
}

// CustomerInput describes a client. Nil fields are left unchanged on updates.
type CustomerInput struct {
	Name              *string `json:"name,omitempty" jsonschema:"Company or person name"`
	Professional      *bool   `json:"professional,omitempty" jsonschema:"true for a company, false for an individual"`
	Email             *string `json:"email,omitempty" jsonschema:"Billing email"`
	Phone             *string `json:"phone,omitempty" jsonschema:"Phone number"`
	Address           *string `json:"address,omitempty" jsonschema:"Street address"`
	AddressComplement *string `json:"address_complement,omitempty" jsonschema:"Address complement"`
	PostalCode        *string `json:"postal_code,omitempty" jsonschema:"Postal code"`
	City              *string `json:"city,omitempty" jsonschema:"City"`
	CountryCode       *string `json:"country_code,omitempty" jsonschema:"ISO country code, e.g. FR"`
	SirenOrSiret      *string `json:"siren_or_siret,omitempty" jsonschema:"SIREN or SIRET number"`
	IntracomVATNumber *string `json:"intracom_vat_number,omitempty" jsonschema:"Intra-community VAT number"`
	Note              *string `json:"note,omitempty" jsonschema:"Internal note"`
}

// layoutSettings are copied from a reference document so a new draft keeps the
// seller details and layout already used by the company.
var layoutSettings = []string{
	"template", "color", "header", "footer", "free_field", "free_field_enabled",
	"lang_country_code", "invoicing_logo", "seller", "number_template",
	"non_applicable_vat_reason", "non_applicable_vat_reason_enabled",
	"title_enabled", "units_enabled", "vat_on_debits_enabled",
}

// invoiceSettings adds the payment terms and bank details of invoices.
var invoiceSettings = append(slices.Clone(layoutSettings),
	"bank_detail_enabled", "iban", "bic", "payment_condition", "payment_condition_enabled",
	"recovery_fee", "early_payment_discount", "due_date_mode",
)

// invoiceDraftFields are the other fields the web application sends when it
// saves an invoice. Read-only fields of the invoice detail are left out.
var invoiceDraftFields = []string{
	"status", "type", "client", "client_name", "client_address", "client_address_complement",
	"client_postal_code", "client_city", "client_country_code", "client_siren_or_siret",
	"client_siren_or_siret_enabled", "client_intracom_vat_number", "client_intracom_vat_number_enabled",
	"client_pa_electronic_address", "client_delivery_enabled", "client_delivery_name",
	"client_delivery_address", "client_delivery_postal_code", "client_delivery_city",
	"client_delivery_country_code", "emission_date", "due_date", "title", "comment",
	"lines", "text_lines", "total_excluding_taxes", "total_including_taxes", "totals_per_vat_type",
	"discount_amount", "discount_description", "discount_percentage", "discount_enabled",
	"advance_payment_invoices_ids", "billing_period_start_date", "billing_period_end_date",
}

// invoiceClientFields maps the client snapshot stored on an invoice to the
// client fields.
var invoiceClientFields = map[string]string{
	"client_name":                "name",
	"client_address":             "address",
	"client_address_complement":  "address_complement",
	"client_postal_code":         "postal_code",
	"client_city":                "city",
	"client_country_code":        "country_code",
	"client_siren_or_siret":      "siren_or_siret",
	"client_intracom_vat_number": "intracom_vat_number",
}

// VATTypes lists the VAT types usable on invoice lines.
func (c *Client) VATTypes(
	ctx context.Context,
) ([]VATType, error) {
	var types []VATType
	_, err := c.getJSON(ctx, "/invoice/invoicing_config/vat_types", nil, nil, &types)

	return types, err
}

// salesDocument describes how invoices and quotes are saved: both share the
// same contract, with a few fields of their own.
type salesDocument struct {
	path     string
	settings []string
	fields   []string
	// editable lists the statuses in which the document can still be changed.
	editable []string
}

var (
	invoiceDocument = salesDocument{
		path:     "/invoices",
		settings: invoiceSettings,
		fields:   invoiceDraftFields,
		editable: []string{"draft"},
	}
	quoteDocument = salesDocument{
		path: "/quotations",
		settings: append(slices.Clone(layoutSettings),
			"validity_period", "acceptance_conditions", "acceptance_conditions_enabled", "signature_enabled"),
		fields:   invoiceDraftFields,
		editable: []string{"draft", "saved"},
	}
)

// CreateInvoiceDraft creates an invoice draft for a client, reusing the
// settings of the most recent invoice of the company.
func (c *Client) CreateInvoiceDraft(
	ctx context.Context,
	companyID int64,
	draft InvoiceDraft,
) (json.RawMessage, error) {
	return c.createSalesDraft(ctx, companyID, invoiceDocument, draft, nil)
}

// UpdateInvoiceDraft updates an invoice that is still a draft.
func (c *Client) UpdateInvoiceDraft(
	ctx context.Context,
	companyID int64,
	invoiceID int64,
	draft InvoiceDraft,
) (json.RawMessage, error) {
	return c.updateSalesDocument(ctx, companyID, invoiceDocument, invoiceID, draft, nil)
}

// CreateQuoteDraft creates a quote draft for a client, reusing the settings
// of the most recent quote, or of the most recent invoice for the layout.
func (c *Client) CreateQuoteDraft(
	ctx context.Context,
	companyID int64,
	draft InvoiceDraft,
	validityPeriod string,
) (json.RawMessage, error) {
	extra := map[string]any{}
	if validityPeriod != "" {
		extra["validity_period"] = validityPeriod
	}

	return c.createSalesDraft(ctx, companyID, quoteDocument, draft, extra)
}

// UpdateQuote updates a quote that is a draft or saved but not answered.
// status may set "accepted" or "refused".
func (c *Client) UpdateQuote(
	ctx context.Context,
	companyID int64,
	quoteID int64,
	draft InvoiceDraft,
	validityPeriod string,
	status string,
) (json.RawMessage, error) {
	extra := map[string]any{}
	if validityPeriod != "" {
		extra["validity_period"] = validityPeriod
	}
	if status != "" {
		extra["status"] = status
	}

	return c.updateSalesDocument(ctx, companyID, quoteDocument, quoteID, draft, extra)
}

func (c *Client) createSalesDraft(
	ctx context.Context,
	companyID int64,
	document salesDocument,
	draft InvoiceDraft,
	extra map[string]any,
) (json.RawMessage, error) {
	payload := map[string]any{}

	reference, err := c.latestSalesDocument(ctx, companyID, document)
	if err != nil {
		return nil, err
	}
	if reference == nil && document.path != invoiceDocument.path {
		// Without a previous quote, the layout and seller come from the invoices.
		if reference, err = c.latestSalesDocument(ctx, companyID, invoiceDocument); err != nil {
			return nil, err
		}
	}
	for _, key := range document.settings {
		if value, ok := reference[key]; ok {
			payload[key] = value
		}
	}

	payload["status"] = "draft"
	payload["text_lines"] = []any{}
	payload["discount_enabled"] = false
	payload["client_delivery_enabled"] = false

	if err := c.applyInvoiceDraft(ctx, companyID, payload, draft); err != nil {
		return nil, err
	}
	maps.Copy(payload, extra)

	var raw json.RawMessage
	err = c.sendJSON(ctx, http.MethodPost, salesPath(companyID, document, ""), payload, &raw)

	return raw, err
}

func (c *Client) updateSalesDocument(
	ctx context.Context,
	companyID int64,
	document salesDocument,
	documentID int64,
	draft InvoiceDraft,
	extra map[string]any,
) (json.RawMessage, error) {
	detail, err := c.salesDetail(ctx, companyID, document, documentID)
	if err != nil {
		return nil, err
	}
	if status, _ := detail["status"].(string); !slices.Contains(document.editable, status) {
		return nil, fmt.Errorf("%s %d is %s, it can only be changed when %s",
			strings.TrimSuffix(strings.TrimPrefix(document.path, "/"), "s"), documentID, status, strings.Join(document.editable, " or "))
	}

	payload := map[string]any{}
	for _, key := range append(slices.Clone(document.settings), document.fields...) {
		if value, ok := detail[key]; ok {
			payload[key] = value
		}
	}

	if err := c.applyInvoiceDraft(ctx, companyID, payload, draft); err != nil {
		return nil, err
	}
	maps.Copy(payload, extra)

	var raw json.RawMessage
	err = c.sendJSON(ctx, http.MethodPatch, salesPath(companyID, document, "/"+strconv.FormatInt(documentID, 10)), payload, &raw)

	return raw, err
}

// latestSalesDocument returns the detail of the most recent invoice or quote,
// or nil when there is none. Tiime lists the most recent first by default.
func (c *Client) latestSalesDocument(
	ctx context.Context,
	companyID int64,
	document salesDocument,
) (map[string]any, error) {
	page, err := getPage[idRef](ctx, c, salesPath(companyID, document, ""), nil, nil, Range{Limit: 1}, "")
	if err != nil || len(page.Items) == 0 {
		return nil, err
	}

	return c.salesDetail(ctx, companyID, document, page.Items[0].ID)
}

func (c *Client) salesDetail(
	ctx context.Context,
	companyID int64,
	document salesDocument,
	documentID int64,
) (map[string]any, error) {
	var detail map[string]any
	_, err := c.getJSON(ctx, salesPath(companyID, document, "/"+strconv.FormatInt(documentID, 10)), nil, nil, &detail)

	return detail, err
}

func salesPath(
	companyID int64,
	document salesDocument,
	suffix string,
) string {
	return "/invoice" + companyPath(companyID, document.path+suffix)
}

// InvoiceFromQuote creates an invoice from a quote.
func (c *Client) InvoiceFromQuote(
	ctx context.Context,
	companyID int64,
	quoteID int64,
) (json.RawMessage, error) {
	var raw json.RawMessage
	path := "/invoice" + companyPath(companyID, "/quotations/"+strconv.FormatInt(quoteID, 10)+"/invoice")
	err := c.sendJSON(ctx, http.MethodPost, path, nil, &raw)

	return raw, err
}

// CreateCustomer creates a client.
func (c *Client) CreateCustomer(
	ctx context.Context,
	companyID int64,
	input CustomerInput,
) (json.RawMessage, error) {
	payload := map[string]any{
		"professional":        true,
		"country_code":        "FR",
		"invoicing_use_email": input.Email != nil,
	}
	mergeCustomer(payload, input)

	var raw json.RawMessage
	err := c.sendJSON(ctx, http.MethodPost, "/invoice"+companyPath(companyID, "/clients"), payload, &raw)

	return raw, err
}

// UpdateCustomer updates a client, keeping the fields that are not provided.
func (c *Client) UpdateCustomer(
	ctx context.Context,
	companyID int64,
	customerID int64,
	input CustomerInput,
) (json.RawMessage, error) {
	path := "/invoice" + companyPath(companyID, "/clients/"+strconv.FormatInt(customerID, 10))

	var current map[string]any
	if _, err := c.getJSON(ctx, path, nil, nil, &current); err != nil {
		return nil, err
	}

	payload := map[string]any{}
	for _, key := range []string{
		"professional", "name", "siren_or_siret", "intracom_vat_number", "email", "invoicing_use_email",
		"phone", "address", "address_complement", "postal_code", "city", "country_code", "iban", "bic", "note",
	} {
		payload[key] = current[key]
	}
	mergeCustomer(payload, input)

	var raw json.RawMessage
	err := c.sendJSON(ctx, http.MethodPatch, path, payload, &raw)

	return raw, err
}

func mergeCustomer(
	payload map[string]any,
	input CustomerInput,
) {
	fields := map[string]any{
		"name":                input.Name,
		"professional":        input.Professional,
		"email":               input.Email,
		"phone":               input.Phone,
		"address":             input.Address,
		"address_complement":  input.AddressComplement,
		"postal_code":         input.PostalCode,
		"city":                input.City,
		"country_code":        input.CountryCode,
		"siren_or_siret":      input.SirenOrSiret,
		"intracom_vat_number": input.IntracomVATNumber,
		"note":                input.Note,
	}
	for key, value := range fields {
		switch typed := value.(type) {
		case *string:
			if typed != nil {
				payload[key] = *typed
			}
		case *bool:
			if typed != nil {
				payload[key] = *typed
			}
		}
	}
}

// applyInvoiceDraft sets the client snapshot, dates, lines and totals of an
// invoice payload.
func (c *Client) applyInvoiceDraft(
	ctx context.Context,
	companyID int64,
	payload map[string]any,
	draft InvoiceDraft,
) error {
	if draft.ClientID != 0 {
		var client map[string]any
		if _, err := c.getJSON(ctx, "/invoice"+companyPath(companyID, "/clients/"+strconv.FormatInt(draft.ClientID, 10)), nil, nil, &client); err != nil {
			return fmt.Errorf(
				"loading client %d: %w",
				draft.ClientID,
				err,
			)
		}
		applyClientSnapshot(payload, draft.ClientID, client)
	}

	if draft.EmissionDate != "" {
		payload["emission_date"] = draft.EmissionDate
	}
	if draft.DueDate != "" {
		payload["due_date"] = draft.DueDate
		payload["due_date_mode"] = "due_date"
	}
	if draft.Title != "" {
		payload["title"] = draft.Title
		payload["title_enabled"] = true
	}
	if draft.Comment != "" {
		payload["comment"] = draft.Comment
	}

	if draft.Lines == nil {
		return nil
	}

	emissionDate, _ := payload["emission_date"].(string)
	if emissionDate == "" {
		return fmt.Errorf("emission_date is required to compute the VAT of the lines")
	}

	vatTypes, err := c.VATTypes(ctx)
	if err != nil {
		return err
	}

	lines, totals, err := invoiceLines(draft.Lines, vatTypes, emissionDate)
	if err != nil {
		return err
	}
	payload["lines"] = lines
	for key, value := range totals {
		payload[key] = value
	}

	return nil
}

func applyClientSnapshot(
	payload map[string]any,
	clientID int64,
	client map[string]any,
) {
	payload["client"] = idRef{ID: clientID}
	for invoiceField, clientField := range invoiceClientFields {
		payload[invoiceField] = client[clientField]
	}
	payload["client_siren_or_siret_enabled"] = client["siren_or_siret"] != nil && client["siren_or_siret"] != ""
	payload["client_intracom_vat_number_enabled"] = client["intracom_vat_number"] != nil && client["intracom_vat_number"] != ""
}

// invoiceLines builds the lines of an invoice and its totals, the way the web
// application computes them before saving.
func invoiceLines(
	input []InvoiceLine,
	vatTypes []VATType,
	emissionDate string,
) ([]map[string]any, map[string]any, error) {
	rates := map[string]float64{}
	for _, vatType := range vatTypes {
		if rate, ok := vatType.RateAt(emissionDate); ok {
			rates[vatType.Code] = rate
		}
	}

	lines := make([]map[string]any, 0, len(input))
	perVAT := map[string]map[string]float64{}
	var totalExcluding, totalVAT float64

	for i, line := range input {
		vatType := line.VATType
		if vatType == "" {
			vatType = "normal"
		}
		if _, ok := rates[vatType]; !ok {
			return nil, nil, fmt.Errorf("unknown VAT type %q on line %d", vatType, i+1)
		}

		category := line.CategoryType
		if category == "" {
			category = "benefit"
		}

		amount := round2(line.Quantity * line.UnitAmount)
		lines = append(lines, map[string]any{
			"description":             line.Description,
			"quantity":                line.Quantity,
			"unit_amount":             line.UnitAmount,
			"line_amount":             amount,
			"vat_type":                map[string]string{"code": vatType},
			"invoicing_category_type": category,
			"invoicing_unit":          unitRef(line.UnitID),
			"article":                 nil,
			"discount_amount":         nil,
			"discount_description":    nil,
			"discount_percentage":     nil,
			"sequence":                i + 1,
		})

		if perVAT[vatType] == nil {
			perVAT[vatType] = map[string]float64{}
		}
		perVAT[vatType]["total_excluding_taxes"] += amount
		totalExcluding += amount
	}

	totalsPerVAT := map[string]map[string]float64{}
	for code, totals := range perVAT {
		excluding := round2(totals["total_excluding_taxes"])
		vat := round2(excluding * rates[code])
		totalsPerVAT[code] = map[string]float64{"total_excluding_taxes": excluding, "vat_amount": vat}
		totalVAT += vat
	}

	totals := map[string]any{
		"total_excluding_taxes": round2(totalExcluding),
		"total_including_taxes": round2(totalExcluding + totalVAT),
		"totals_per_vat_type":   totalsPerVAT,
	}

	return lines, totals, nil
}

func round2(
	value float64,
) float64 {
	return math.Round(value*100) / 100
}

func unitRef(
	unitID int64,
) any {
	if unitID == 0 {
		return nil
	}

	return idRef{ID: unitID}
}
