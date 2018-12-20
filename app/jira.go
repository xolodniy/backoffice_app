package app

import (
	"fmt"

	"github.com/andygrunwald/go-jira"
)

// IssuesSearch searches Issues in all sprints which opened now and returning list with issues in this sprints list
func (a *App) IssuesSearch() ([]jira.Issue, *jira.Response, error) {
	// allIssues including issues from other sprints and not closed
	allIssues, response, err := a.Jira.Issue.Search(
		/*`Sprint IN openSprints() AND (status NOT IN ("Closed", "IN PEER REVIEW", "TL REVIEW"))`,*/
		`assignee != "empty" AND Sprint IN openSprints() AND (status NOT IN ("Closed"))`,
		&jira.SearchOptions{
			StartAt:       0,
			MaxResults:    1000,
			ValidateQuery: "strict",
			Fields: []string{
				"customfield_10010", // Sprint
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

func (a *App) IssueTimeExcisionWWithTimeCompare(issue jira.Issue, rowIndex int) (string, error) {
	var listRow string
	if issue.Fields.TimeSpent > issue.Fields.TimeOriginalEstimate {

		ts, err := a.SecondsToClockTime(issue.Fields.TimeSpent)
		te, err := a.SecondsToClockTime(issue.Fields.TimeOriginalEstimate)
		if err != nil {
			return listRow, fmt.Errorf("time conversion: regexp error: %v", err)

		}

		listRow = fmt.Sprintf("%[1]d. <https://theflow.atlassian.net/browse/%[2]s|%[2]s - %[3]s>: %[4]v из %[5]v\n",
			rowIndex, issue.Key, issue.Fields.Summary, ts, te,
		)

	}

	return listRow, nil
}

func (a *App) IssueTimeExcisionNoTimeRange(issue jira.Issue, rowIndex int) string {
	var listRow string
	if issue.Fields.TimeTracking.RemainingEstimateSeconds == 0 {
		listRow = fmt.Sprintf("%[1]d. <https://theflow.atlassian.net/browse/%[2]s|%[2]s - %[3]s>: _%[4]s_\n",
			rowIndex, issue.Key, issue.Fields.Summary, issue.Fields.Status.Name,
		)
	}

	return listRow
}
