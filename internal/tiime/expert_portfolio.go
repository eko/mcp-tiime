package tiime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// ExpertMe returns the accounting firm user that is logged in.
func (c *Client) ExpertMe(
	ctx context.Context,
) (json.RawMessage, error) {
	return c.expertGet(ctx, "/expert/users/me", nil)
}

// ExpertCompanies lists the client companies of the accounting firm.
func (c *Client) ExpertCompanies(
	ctx context.Context,
	query ExpertQuery,
	window Range,
) (RawPage, error) {
	return c.expertList(ctx, "/expert/companies", query.values(), nil, window)
}

// ExpertCompany returns a client company. expand lists optional blocks, such
// as "legal_company,pdp_config,thread".
func (c *Client) ExpertCompany(
	ctx context.Context,
	companyID int64,
	expand string,
) (json.RawMessage, error) {
	query := url.Values{}
	if expand != "" {
		query.Set("expand", expand)
	}

	return c.expertGet(ctx, expertCompanyPath(companyID, ""), query)
}

// ExpertTasks lists the task plannings (missions) of the firm.
func (c *Client) ExpertTasks(
	ctx context.Context,
	query ExpertQuery,
	window Range,
) (RawPage, error) {
	return c.expertList(ctx, "/expert/users/me/tasks", query.values(), nil, window)
}

// ExpertCompanyTasks lists every task planning of one client company.
func (c *Client) ExpertCompanyTasks(
	ctx context.Context,
	companyID int64,
) ([]json.RawMessage, error) {
	query := url.Values{"company": {strconv.FormatInt(companyID, 10)}}

	var tasks []json.RawMessage
	_, err := c.getJSON(ctx, "/expert/users/me/tasks", query, nil, &tasks)

	return tasks, err
}

// ExpertTaskTotals sums the amounts of the task plannings matching query.
func (c *Client) ExpertTaskTotals(
	ctx context.Context,
	query ExpertQuery,
) (json.RawMessage, error) {
	query.Sort = ""

	return c.expertGet(ctx, "/expert/users/me/tasks/totals", query.values())
}

// CreateExpertTask creates a task planning. task is the JSON body of the
// expert application.
func (c *Client) CreateExpertTask(
	ctx context.Context,
	task map[string]any,
) (json.RawMessage, error) {
	return c.expertSend(ctx, http.MethodPost, "/expert/tasks", nil, task)
}

// UpdateExpertTask replaces a task planning with task, its full JSON body.
// applyNexts, the task deadline date, also updates the next occurrences of a
// recurring task when not empty.
func (c *Client) UpdateExpertTask(
	ctx context.Context,
	companyID int64,
	taskID int64,
	task map[string]any,
	applyNexts string,
) (json.RawMessage, error) {
	return c.expertSend(ctx, http.MethodPatch, expertCompanyPath(companyID, "/tasks"+idPath(taskID)), applyNextsQuery(applyNexts), task)
}

// DeleteExpertTask deletes a task planning, and its next occurrences when
// applyNexts is the task deadline date.
func (c *Client) DeleteExpertTask(
	ctx context.Context,
	companyID int64,
	taskID int64,
	applyNexts string,
) error {
	return c.expertDelete(ctx, expertCompanyPath(companyID, "/tasks"+idPath(taskID)), applyNextsQuery(applyNexts))
}

// SetExpertTaskBillingPointed marks a task planning as checked for billing,
// or removes the mark.
func (c *Client) SetExpertTaskBillingPointed(
	ctx context.Context,
	companyID int64,
	taskID int64,
	pointed bool,
) (json.RawMessage, error) {
	action := "/billing_unpoint"
	if pointed {
		action = "/billing_point"
	}

	return c.expertSend(ctx, http.MethodPost, expertCompanyPath(companyID, "/tasks"+idPath(taskID)+action), nil, map[string]any{})
}

func applyNextsQuery(
	applyNexts string,
) url.Values {
	if applyNexts == "" {
		return nil
	}

	return url.Values{"apply_nexts": {applyNexts}}
}

// TimeTrackingReference lists one kind of time tracking reference data:
// "categories", "tasks", "invoicing_types" or "duration_types". filters
// applies to tasks, e.g. {"time_tracking": "true"}.
func (c *Client) TimeTrackingReference(
	ctx context.Context,
	kind string,
	filters map[string]string,
) (json.RawMessage, error) {
	return c.expertGet(ctx, "/expert/time_tracking/"+url.PathEscape(kind), ExpertQuery{Filters: filters}.values())
}

// TimeRecords lists the time records of a user for a day.
func (c *Client) TimeRecords(
	ctx context.Context,
	userID int64,
	date string,
) (json.RawMessage, error) {
	query := url.Values{"user_id": {strconv.FormatInt(userID, 10)}, "date": {date}}

	return c.expertGet(ctx, "/expert/time_tracking/records", query)
}

// TimeDays summarizes the recorded and validated time of users per day.
func (c *Client) TimeDays(
	ctx context.Context,
	ownerIDs string,
	startDate string,
	endDate string,
) (json.RawMessage, error) {
	query := url.Values{"owner_ids": {ownerIDs}, "start_date": {startDate}, "end_date": {endDate}}

	return c.expertGet(ctx, "/expert/time_tracking/records/days", query)
}

// CompanyTimeRecords lists the time recorded on a client company.
func (c *Client) CompanyTimeRecords(
	ctx context.Context,
	companyID int64,
	query ExpertQuery,
	window Range,
) (RawPage, error) {
	return c.expertList(ctx, expertCompanyPath(companyID, "/time_tracking/records"), query.values(), nil, window)
}

// CompanyTimeTotal sums the time recorded on a client company, in minutes.
func (c *Client) CompanyTimeTotal(
	ctx context.Context,
	companyID int64,
	query ExpertQuery,
) (json.RawMessage, error) {
	query.Sort = ""

	return c.expertGet(ctx, expertCompanyPath(companyID, "/time_tracking/records/totals"), query.values())
}

// SaveTimeRecord creates a time record, or updates it when record has an id.
func (c *Client) SaveTimeRecord(
	ctx context.Context,
	record map[string]any,
) (json.RawMessage, error) {
	return c.expertSend(ctx, http.MethodPut, "/expert/time_tracking/records", nil, record)
}

// DeleteTimeRecord deletes a time record.
func (c *Client) DeleteTimeRecord(
	ctx context.Context,
	recordID int64,
) error {
	return c.expertDelete(ctx, "/expert/time_tracking/records"+idPath(recordID), nil)
}

// ValidateTimeRecords validates, or unvalidates, the time of users between
// two dates.
func (c *Client) ValidateTimeRecords(
	ctx context.Context,
	userIDs []int64,
	startDate string,
	endDate string,
	validated bool,
) error {
	payload := map[string]any{
		"start_date": startDate,
		"end_date":   endDate,
		"user_ids":   userIDs,
		"validated":  validated,
	}
	_, err := c.expertSend(ctx, http.MethodPost, "/expert/time_tracking/records/validate", nil, payload)

	return err
}
