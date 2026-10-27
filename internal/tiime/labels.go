package tiime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const labelsAccept = "application/vnd.tiime.labels.v2+json"

// ErrNotFoundInList is returned when an item is missing from a listing.
var ErrNotFoundInList = errors.New("not found")

// SearchLabels lists the accounting labels of the company, optionally
// matching search, with the number of categorization rules of each. Tiime
// returns at most 100 labels per page.
func (c *Client) SearchLabels(
	ctx context.Context,
	companyID int64,
	search string,
) ([]Label, error) {
	query := url.Values{"with_rules_count": {"true"}}
	if search != "" {
		query.Set("q", strings.ToLower(search))
	}
	headers := map[string]string{"Accept": labelsAccept}

	var labels []Label
	for offset := 0; offset < 1000; offset += 100 {
		page, err := getPage[Label](ctx, c, accountsPath(companyID, "/labels"), query, headers, Range{Offset: offset, Limit: 100}, "")
		if err != nil {
			return nil, err
		}
		labels = append(labels, page.Items...)
		if !page.HasMore || len(page.Items) == 0 {
			break
		}
	}

	return labels, nil
}

// StandardLabels lists the standard accounting labels that can be adopted.
func (c *Client) StandardLabels(
	ctx context.Context,
	companyID int64,
) ([]Label, error) {
	var labels []Label
	_, err := c.getJSON(ctx, accountsPath(companyID, "/standard_labels"), nil, nil, &labels)

	return labels, err
}

// CreateLabel adopts a standard label, or creates a custom label when the
// company allows it. label is sent the way the web application does, with
// the property names of its model.
func (c *Client) CreateLabel(
	ctx context.Context,
	companyID int64,
	label Label,
) (Label, error) {
	payload := map[string]any{
		"label":        label.Label,
		"name":         label.Name,
		"color":        label.Color,
		"acronym":      label.Acronym,
		"disabled":     false,
		"registryType": label.RegistryType,
		"isStandard":   label.IsStandard,
	}

	var created Label
	_, err := c.callJSON(ctx, http.MethodPost, accountsPath(companyID, "/labels"), payload, map[string]string{"Accept": labelsAccept}, &created)

	return created, err
}

// RenameLabel renames a custom label.
func (c *Client) RenameLabel(
	ctx context.Context,
	companyID int64,
	labelID int64,
	name string,
) (Label, error) {
	return c.patchLabel(ctx, companyID, labelID, map[string]any{"id": labelID, "label": name})
}

// SetLabelDisabled archives or reactivates a label.
func (c *Client) SetLabelDisabled(
	ctx context.Context,
	companyID int64,
	labelID int64,
	disabled bool,
) (Label, error) {
	return c.patchLabel(ctx, companyID, labelID, map[string]any{"id": labelID, "disabled": disabled})
}

func (c *Client) patchLabel(
	ctx context.Context,
	companyID int64,
	labelID int64,
	payload map[string]any,
) (Label, error) {
	var label Label
	path := accountsPath(companyID, "/labels/"+strconv.FormatInt(labelID, 10))
	_, err := c.callJSON(ctx, http.MethodPatch, path, payload, map[string]string{"Accept": labelsAccept}, &label)

	return label, err
}

// LabelRules lists the automatic categorization rules.
func (c *Client) LabelRules(
	ctx context.Context,
	companyID int64,
) ([]LabelRule, error) {
	var rules []LabelRule
	_, err := c.getJSON(ctx, companyPath(companyID, "/label_rules"), nil, map[string]string{"Range": "items=0-499"}, &rules)

	return rules, err
}

// CreateLabelRule applies a label to the transactions whose wording
// contains wording.
func (c *Client) CreateLabelRule(
	ctx context.Context,
	companyID int64,
	labelID int64,
	wording string,
) (LabelRule, error) {
	var rule LabelRule
	payload := map[string]any{"label": idRef{ID: labelID}, "wording": wording}
	err := c.sendJSON(ctx, http.MethodPost, companyPath(companyID, "/label_rules"), payload, &rule)

	return rule, err
}

// DeleteLabelRule deletes a categorization rule.
func (c *Client) DeleteLabelRule(
	ctx context.Context,
	companyID int64,
	ruleID int64,
) error {
	_, err := c.call(ctx, request{method: http.MethodDelete, path: companyPath(companyID, "/label_rules/"+strconv.FormatInt(ruleID, 10))}, nil)

	return err
}

// AllTags lists every tag of the company, archived ones included.
func (c *Client) AllTags(
	ctx context.Context,
	companyID int64,
) ([]Tag, error) {
	var tags []Tag
	query := url.Values{"expand": {"tag_detail"}}
	_, err := c.getJSON(ctx, accountsPath(companyID, "/tags"), query, map[string]string{"Range": "items=0-999"}, &tags)

	return tags, err
}

// UpdateTag renames a tag and or archives it. Tiime expects the whole tag,
// with the property names of the web application model.
func (c *Client) UpdateTag(
	ctx context.Context,
	companyID int64,
	tagID int64,
	name string,
	archived *bool,
) (Tag, error) {
	tags, err := c.AllTags(ctx, companyID)
	if err != nil {
		return Tag{}, err
	}

	var current *Tag
	for i := range tags {
		if tags[i].ID == tagID {
			current = &tags[i]
		}
	}
	if current == nil {
		return Tag{}, fmt.Errorf("tag %d: %w", tagID, ErrNotFoundInList)
	}

	payload := map[string]any{
		"id":          current.ID,
		"name":        current.Name,
		"type":        nilIfEmpty(current.Type),
		"isAutomatic": current.IsAutomatic,
		"archivedAt":  nilIfEmpty(current.ArchivedAt),
	}
	if name != "" {
		payload["name"] = name
	}
	if archived != nil {
		payload["archivedAt"] = nil
		if *archived {
			payload["archivedAt"] = time.Now().UTC().Format(time.RFC3339)
		}
	}

	var tag Tag
	err = c.sendJSON(ctx, http.MethodPatch, accountsPath(companyID, "/tags/"+strconv.FormatInt(tagID, 10)), payload, &tag)

	return tag, err
}

// DeleteTag deletes a tag.
func (c *Client) DeleteTag(
	ctx context.Context,
	companyID int64,
	tagID int64,
) error {
	_, err := c.call(ctx, request{method: http.MethodDelete, path: accountsPath(companyID, "/tags/"+strconv.FormatInt(tagID, 10))}, nil)

	return err
}

// CreateDocumentCategory creates a custom category in the documents space.
func (c *Client) CreateDocumentCategory(
	ctx context.Context,
	companyID int64,
	name string,
) (DocumentCategory, error) {
	var category DocumentCategory
	err := c.sendJSON(ctx, http.MethodPost, accountsPath(companyID, "/document_categories"), map[string]string{"name": name}, &category)

	return category, err
}

// DeleteDocumentCategory deletes a custom category of the documents space.
func (c *Client) DeleteDocumentCategory(
	ctx context.Context,
	companyID int64,
	categoryID int64,
) error {
	_, err := c.call(ctx, request{method: http.MethodDelete, path: accountsPath(companyID, "/document_categories/"+strconv.FormatInt(categoryID, 10))}, nil)

	return err
}

// callJSON sends payload as JSON with extra headers and decodes the response.
func (c *Client) callJSON(
	ctx context.Context,
	method string,
	path string,
	payload any,
	headers map[string]string,
	out any,
) (any, error) {
	body, err := jsonBody(payload)
	if err != nil {
		return nil, err
	}

	_, err = c.call(ctx, request{method: method, path: path, headers: headers, body: body, contentType: "application/json"}, out)

	return out, err
}

func nilIfEmpty(
	value string,
) any {
	if value == "" {
		return nil
	}

	return value
}
