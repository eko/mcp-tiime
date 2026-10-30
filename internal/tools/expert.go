package tools

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ExpertAPI is the subset of the Tiime client used by the expert tools.
type ExpertAPI interface {
	ExpertPortfolioAPI
	ExpertAccountingAPI
	ExpertFollowUpAPI
}

// ExpertHandler implements the MCP tools of the expert mode, for accounting
// firms using expert.tiime.fr.
type ExpertHandler struct {
	api              ExpertAPI
	auth             Auth
	defaultCompanyID int64
	downloadDir      string
}

// NewExpertHandler creates an ExpertHandler. defaultCompanyID, when not zero,
// is the client company used by the tools that are not given one.
func NewExpertHandler(
	api ExpertAPI,
	auth Auth,
	defaultCompanyID int64,
	downloadDir string,
) *ExpertHandler {
	return &ExpertHandler{
		api:              api,
		auth:             auth,
		defaultCompanyID: defaultCompanyID,
		downloadDir:      downloadDir,
	}
}

// RegisterExpert adds the expert mode tools to the MCP server.
func RegisterExpert(
	server *mcp.Server,
	h *ExpertHandler,
) {
	registerAuth(server, h.auth)
	registerExpertPortfolio(server, h)
	registerExpertAccounting(server, h)
	registerExpertFollowUp(server, h)
}

// ExpertCompanyInput selects the client company of the firm a tool works on.
type ExpertCompanyInput struct {
	CompanyID int64 `json:"company_id,omitempty" jsonschema:"ID of the client company, see list_portfolio_companies. Optional when TIIME_COMPANY_ID is set"`
}

// errCompanyRequired is returned when an expert tool needs a client company.
var errCompanyRequired = errors.New("company_id is required: find it with list_portfolio_companies")

func (h *ExpertHandler) company(
	requested int64,
) (int64, error) {
	if requested != 0 {
		return requested, nil
	}
	if h.defaultCompanyID != 0 {
		return h.defaultCompanyID, nil
	}

	return 0, errCompanyRequired
}

// dateRange builds the ">=from,<=to" filter of the expert application.
func dateRange(
	from string,
	to string,
) (string, error) {
	for _, date := range []string{from, to} {
		if err := checkDate(date); err != nil {
			return "", err
		}
	}

	var parts []string
	if from != "" {
		parts = append(parts, ">="+from)
	}
	if to != "" {
		parts = append(parts, "<="+to)
	}

	return strings.Join(parts, ","), nil
}

// checkDate accepts an empty value or a YYYY-MM-DD date.
func checkDate(
	date string,
) error {
	if date == "" {
		return nil
	}
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return fmt.Errorf("invalid date %q, expected YYYY-MM-DD", date)
	}

	return nil
}

// joinIDs joins IDs with sep, "|" for most expert filters.
func joinIDs(
	ids []int64,
	sep string,
) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}

	return strings.Join(parts, sep)
}

// boolParam renders an optional boolean filter, empty when unset.
func boolParam(
	value *bool,
) string {
	if value == nil {
		return ""
	}

	return strconv.FormatBool(*value)
}

// checkEnum accepts an empty value or one of allowed.
func checkEnum(
	field string,
	value string,
	allowed []string,
) error {
	if value == "" || slices.Contains(allowed, value) {
		return nil
	}

	return fmt.Errorf("invalid %s %q, expected one of: %s", field, value, strings.Join(allowed, ", "))
}

// sortParam checks "<field>:<asc|desc>" against the sortable fields.
func sortParam(
	sort string,
	fields []string,
) (string, error) {
	if sort == "" {
		return "", nil
	}

	field, direction, _ := strings.Cut(sort, ":")
	if direction == "" {
		direction = "asc"
	}
	if !slices.Contains(fields, field) || (direction != "asc" && direction != "desc") {
		return "", fmt.Errorf("invalid sort %q, expected <field>:<asc|desc> with field one of: %s", sort, strings.Join(fields, ", "))
	}

	return field + ":" + direction, nil
}

func idRef(
	id int64,
) map[string]any {
	if id == 0 {
		return nil
	}

	return map[string]any{"id": id}
}
