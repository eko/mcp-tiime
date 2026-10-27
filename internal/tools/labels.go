package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// LabelsAPI manages accounting labels, tags, categorization rules and
// document categories.
type LabelsAPI interface {
	SearchLabels(
		ctx context.Context,
		companyID int64,
		search string,
	) ([]tiime.Label, error)
	StandardLabels(
		ctx context.Context,
		companyID int64,
	) ([]tiime.Label, error)
	CreateLabel(
		ctx context.Context,
		companyID int64,
		label tiime.Label,
	) (tiime.Label, error)
	RenameLabel(
		ctx context.Context,
		companyID int64,
		labelID int64,
		name string,
	) (tiime.Label, error)
	SetLabelDisabled(
		ctx context.Context,
		companyID int64,
		labelID int64,
		disabled bool,
	) (tiime.Label, error)
	LabelRules(
		ctx context.Context,
		companyID int64,
	) ([]tiime.LabelRule, error)
	CreateLabelRule(
		ctx context.Context,
		companyID int64,
		labelID int64,
		wording string,
	) (tiime.LabelRule, error)
	DeleteLabelRule(
		ctx context.Context,
		companyID int64,
		ruleID int64,
	) error
	AllTags(
		ctx context.Context,
		companyID int64,
	) ([]tiime.Tag, error)
	UpdateTag(
		ctx context.Context,
		companyID int64,
		tagID int64,
		name string,
		archived *bool,
	) (tiime.Tag, error)
	DeleteTag(
		ctx context.Context,
		companyID int64,
		tagID int64,
	) error
	CreateDocumentCategory(
		ctx context.Context,
		companyID int64,
		name string,
	) (tiime.DocumentCategory, error)
	DeleteDocumentCategory(
		ctx context.Context,
		companyID int64,
		categoryID int64,
	) error
}

// ListLabelsInput searches labels.
type ListLabelsInput struct {
	CompanyInput
	Search          string `json:"search,omitempty" jsonschema:"Text the label name must contain"`
	IncludeDisabled bool   `json:"include_disabled,omitempty" jsonschema:"true to include archived labels"`
}

// CreateLabelInput describes a label to add.
type CreateLabelInput struct {
	CompanyInput
	Standard string `json:"standard,omitempty" jsonschema:"Name of a standard label to adopt, see list_standard_labels"`
	Name     string `json:"name,omitempty" jsonschema:"Name of a custom label, when the accounting firm allows custom labels"`
	Color    string `json:"color,omitempty" jsonschema:"Color of a custom label, e.g. #85B1FF"`
}

// UpdateLabelInput describes changes on a label.
type UpdateLabelInput struct {
	CompanyInput
	ID       int64  `json:"id" jsonschema:"ID of the label"`
	Name     string `json:"name,omitempty" jsonschema:"New name, only for custom labels"`
	Disabled *bool  `json:"disabled,omitempty" jsonschema:"true to archive the label, false to reactivate it"`
}

// CreateLabelRuleInput describes a categorization rule.
type CreateLabelRuleInput struct {
	CompanyInput
	LabelID int64  `json:"label_id" jsonschema:"Label applied by the rule, see list_labels"`
	Wording string `json:"wording" jsonschema:"Text the transaction wording must contain, e.g. URSSAF"`
}

// ListTagsInput filters tags.
type ListTagsInput struct {
	CompanyInput
	Search          string `json:"search,omitempty" jsonschema:"Text the tag name must contain"`
	IncludeArchived bool   `json:"include_archived,omitempty" jsonschema:"true to include archived tags"`
}

// UpdateTagInput describes changes on a tag.
type UpdateTagInput struct {
	CompanyInput
	ID       int64  `json:"id" jsonschema:"ID of the tag"`
	Name     string `json:"name,omitempty" jsonschema:"New name"`
	Archived *bool  `json:"archived,omitempty" jsonschema:"true to archive the tag, false to restore it"`
}

// CreateCategoryInput describes a document category.
type CreateCategoryInput struct {
	CompanyInput
	Name string `json:"name" jsonschema:"Name of the category"`
}

// LabelOutput is a single label.
type LabelOutput struct {
	Label tiime.Label `json:"label"`
}

// LabelRulesOutput lists categorization rules.
type LabelRulesOutput struct {
	Rules []tiime.LabelRule `json:"rules"`
}

// TagOutput is a single tag.
type TagOutput struct {
	Tag tiime.Tag `json:"tag"`
}

func registerLabels(
	server *mcp.Server,
	h *Handler,
) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := true

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_labels",
		Description: "List the accounting labels (categories) of the company, with the number of categorization rules of each.",
		Annotations: readOnly,
	}, h.ListLabels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_standard_labels",
		Description: "List the standard accounting labels that can be added to the company with create_label.",
		Annotations: readOnly,
	}, h.ListStandardLabels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_label",
		Description: "Add an accounting label: adopt a standard label by name, or create a custom label when the accounting firm allows it.",
	}, h.CreateLabel)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_label",
		Description: "Rename a custom accounting label, or archive or reactivate a label.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.UpdateLabel)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_label_rules",
		Description: "List the automatic categorization rules: transactions whose wording contains a text get a label.",
		Annotations: readOnly,
	}, h.ListLabelRules)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_label_rule",
		Description: "Create an automatic categorization rule: transactions whose wording contains the text get the label.",
	}, h.CreateLabelRule)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_label_rule",
		Description: "Delete an automatic categorization rule.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.DeleteLabelRule)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_tags",
		Description: "List the tags of the company. Tags are created by applying a new name with annotate_bank_transactions.",
		Annotations: readOnly,
	}, h.ListTags)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_tag",
		Description: "Rename a tag, or archive or restore it.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, h.UpdateTag)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_tag",
		Description: "Delete a tag. Tiime refuses it for tags used in accounting entries: archive them instead.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.DeleteTag)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_document_category",
		Description: "Create a custom category in the documents space.",
	}, h.CreateDocumentCategory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_document_category",
		Description: "Delete a custom category of the documents space.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, h.DeleteDocumentCategory)
}

// ListLabels implements the list_labels tool.
func (h *Handler) ListLabels(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListLabelsInput,
) (*mcp.CallToolResult, LabelsOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, LabelsOutput{}, err
	}

	labels, err := h.api.SearchLabels(ctx, companyID, in.Search)
	if err != nil {
		return nil, LabelsOutput{}, err
	}

	output := LabelsOutput{Labels: []tiime.Label{}}
	for _, label := range labels {
		if in.IncludeDisabled || !label.Disabled {
			output.Labels = append(output.Labels, label)
		}
	}

	return nil, output, nil
}

// ListStandardLabels implements the list_standard_labels tool.
func (h *Handler) ListStandardLabels(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CompanyInput,
) (*mcp.CallToolResult, LabelsOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, LabelsOutput{}, err
	}

	labels, err := h.api.StandardLabels(ctx, companyID)

	return nil, LabelsOutput{Labels: labels}, err
}

// CreateLabel implements the create_label tool.
func (h *Handler) CreateLabel(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CreateLabelInput,
) (*mcp.CallToolResult, LabelOutput, error) {
	if (in.Standard == "") == (in.Name == "") {
		return nil, LabelOutput{}, fmt.Errorf("set either standard or name")
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, LabelOutput{}, err
	}

	label := tiime.Label{Label: in.Name, Name: in.Name, Color: in.Color}
	if in.Standard != "" {
		label, err = h.findStandardLabel(ctx, companyID, in.Standard)
		if err != nil {
			return nil, LabelOutput{}, err
		}
	}

	created, err := h.api.CreateLabel(ctx, companyID, label)

	return nil, LabelOutput{Label: created}, err
}

func (h *Handler) findStandardLabel(
	ctx context.Context,
	companyID int64,
	name string,
) (tiime.Label, error) {
	standards, err := h.api.StandardLabels(ctx, companyID)
	if err != nil {
		return tiime.Label{}, err
	}

	for _, standard := range standards {
		if strings.EqualFold(standard.Label, name) || strings.EqualFold(standard.Name, name) {
			standard.IsStandard = true
			return standard, nil
		}
	}

	return tiime.Label{}, fmt.Errorf("no standard label named %q, see list_standard_labels", name)
}

// UpdateLabel implements the update_label tool.
func (h *Handler) UpdateLabel(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in UpdateLabelInput,
) (*mcp.CallToolResult, LabelOutput, error) {
	if in.Name == "" && in.Disabled == nil {
		return nil, LabelOutput{}, fmt.Errorf("set name or disabled")
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, LabelOutput{}, err
	}

	var label tiime.Label
	if in.Name != "" {
		if label, err = h.api.RenameLabel(ctx, companyID, in.ID, in.Name); err != nil {
			return nil, LabelOutput{}, err
		}
	}
	if in.Disabled != nil {
		if label, err = h.api.SetLabelDisabled(ctx, companyID, in.ID, *in.Disabled); err != nil {
			return nil, LabelOutput{}, err
		}
	}

	return nil, LabelOutput{Label: label}, nil
}

// ListLabelRules implements the list_label_rules tool.
func (h *Handler) ListLabelRules(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CompanyInput,
) (*mcp.CallToolResult, LabelRulesOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, LabelRulesOutput{}, err
	}

	rules, err := h.api.LabelRules(ctx, companyID)

	return nil, LabelRulesOutput{Rules: rules}, err
}

// CreateLabelRule implements the create_label_rule tool.
func (h *Handler) CreateLabelRule(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CreateLabelRuleInput,
) (*mcp.CallToolResult, LabelRulesOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, LabelRulesOutput{}, err
	}

	rule, err := h.api.CreateLabelRule(ctx, companyID, in.LabelID, in.Wording)

	return nil, LabelRulesOutput{Rules: []tiime.LabelRule{rule}}, err
}

// DeleteLabelRule implements the delete_label_rule tool.
func (h *Handler) DeleteLabelRule(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, DeleteOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, DeleteOutput{}, err
	}

	if err := h.api.DeleteLabelRule(ctx, companyID, in.ID); err != nil {
		return nil, DeleteOutput{}, err
	}

	return nil, DeleteOutput{Deleted: in.ID}, nil
}

// ListTags implements the list_tags tool.
func (h *Handler) ListTags(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in ListTagsInput,
) (*mcp.CallToolResult, TagsOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, TagsOutput{}, err
	}

	tags, err := h.api.AllTags(ctx, companyID)
	if err != nil {
		return nil, TagsOutput{}, err
	}

	output := TagsOutput{Tags: []tiime.Tag{}}
	for _, tag := range tags {
		if (in.IncludeArchived || tag.ArchivedAt == "") &&
			strings.Contains(strings.ToLower(tag.Name), strings.ToLower(in.Search)) {
			output.Tags = append(output.Tags, tag)
		}
	}

	return nil, output, nil
}

// UpdateTag implements the update_tag tool.
func (h *Handler) UpdateTag(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in UpdateTagInput,
) (*mcp.CallToolResult, TagOutput, error) {
	if in.Name == "" && in.Archived == nil {
		return nil, TagOutput{}, fmt.Errorf("set name or archived")
	}

	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, TagOutput{}, err
	}

	tag, err := h.api.UpdateTag(ctx, companyID, in.ID, in.Name, in.Archived)

	return nil, TagOutput{Tag: tag}, err
}

// DeleteTag implements the delete_tag tool.
func (h *Handler) DeleteTag(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, DeleteOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, DeleteOutput{}, err
	}

	if err := h.api.DeleteTag(ctx, companyID, in.ID); err != nil {
		return nil, DeleteOutput{}, err
	}

	return nil, DeleteOutput{Deleted: in.ID}, nil
}

// CreateDocumentCategory implements the create_document_category tool.
func (h *Handler) CreateDocumentCategory(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CreateCategoryInput,
) (*mcp.CallToolResult, tiime.DocumentCategory, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, tiime.DocumentCategory{}, err
	}

	category, err := h.api.CreateDocumentCategory(ctx, companyID, in.Name)

	return nil, category, err
}

// DeleteDocumentCategory implements the delete_document_category tool.
func (h *Handler) DeleteDocumentCategory(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in GetByIDInput,
) (*mcp.CallToolResult, DeleteOutput, error) {
	companyID, err := h.resolveCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, DeleteOutput{}, err
	}

	if err := h.api.DeleteDocumentCategory(ctx, companyID, in.ID); err != nil {
		return nil, DeleteOutput{}, err
	}

	return nil, DeleteOutput{Deleted: in.ID}, nil
}
