package app

import (
	"fmt"

	"backoffice_app/services/util"
	"github.com/andygrunwald/go-jira"
	"github.com/sirupsen/logrus"
)

// IssuesSearch searches Issues in all sprints which opened now and returning list with issues in this sprints list
func (a *App) IssuesSearch() ([]jira.Issue, *jira.Response, error) {
	allIssues, response, err := a.Jira.Issue.Search(
		`assignee != "empty" AND Sprint IN openSprints() AND (status NOT IN ("Closed")) AND issuetype IN subTaskIssueTypes()`,
		&jira.SearchOptions{
			StartAt:       0,
			MaxResults:    1000,
			ValidateQuery: "strict",
			Fields: []string{
				"customfield_10026",
				"timetracking",
				"timespent",
				"timeoriginalestimate",
				"summary",
				"status",
				"issuetype",
			},
		},
	)

	if err != nil {
		return nil, response, fmt.Errorf("can't create jira client: %s", err)
	}

	return allIssues, response, nil
}

// IssueTimeExcisionWithTimeCompare prepares string with employee time excess
// Not used now, but will be used further.
func (a *App) IssueTimeExcisionWithTimeCompare(issue jira.Issue, rowIndex int) (string, error) {
	if issue.Fields == nil {
		logrus.WithField("issue", issue).Error("issue fields is empty")
		return "", fmt.Errorf("issue fields is empty")
	}
	var listRow string
	if issue.Fields.TimeSpent < issue.Fields.TimeOriginalEstimate {
		return listRow, nil
	}

	ts, err := util.DurationStringInHoursMinutes(issue.Fields.TimeSpent)
	if err != nil {
		logrus.WithError(err).WithField("time", issue.Fields.TimeSpent).
			Error("error occurred on time conversion error")
		return listRow, fmt.Errorf("time conversion: %v", err)

	}

	te, err := util.DurationStringInHoursMinutes(issue.Fields.TimeOriginalEstimate)
	if err != nil {
		logrus.WithError(err).WithField("time", issue.Fields.TimeOriginalEstimate).
			Error("error occurred on time conversion error")
		return listRow, fmt.Errorf("time conversion: %v", err)
	}

	listRow = fmt.Sprintf("%[1]d. <https://theflow.atlassian.net/browse/%[2]s|%[2]s - %[3]s>: %[4]v из %[5]v\n",
		rowIndex, issue.Key, issue.Fields.Summary, ts, te,
	)

	return listRow, nil
}

// IssueTimeExceededNoTimeRange prepares string without employee time excess
func (a *App) IssueTimeExceededNoTimeRange(issue jira.Issue, rowIndex int) string {
	if issue.Fields == nil {
		logrus.WithField("issue", issue).Error("issue fields is empty")
		return ""
	}

	var listRow string
	if issue.Fields.TimeTracking.RemainingEstimateSeconds != 0 {
		return listRow
	}

	var developer = "No developer"
	developerMap, err := issue.Fields.Unknowns.MarshalMap("customfield_10026")
	if err != nil {
		logrus.WithError(err).WithField("developerMap", developerMap).
			Error("can't make customfield_10026 map marshaling")
	} else if developerMap != nil {
		displayName, ok := developerMap["displayName"].(string)
		if !ok {
			logrus.WithField("displayName", developerMap["displayName"]).
				Error("can't assert to string map displayName field")
		} else {
			developer = displayName
		}
	}

	listRow = fmt.Sprintf("%[1]d. %[2]s - <https://theflow.atlassian.net/browse/%[3]s|%[3]s - %[4]s>: _%[5]s_\n",
		rowIndex, developer, issue.Key, issue.Fields.Summary, issue.Fields.Status.Name,
	)

	return listRow
}
