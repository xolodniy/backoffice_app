package app

import (
	"fmt"

	"github.com/andygrunwald/go-jira"
)

// IssuesSearch searches Issues in all sprints which opened now and returning list with issues in this sprints list
func (a *App) IssuesSearch() ([]jira.Issue, *jira.Response, error) {
	// allIssues including issues from other sprints and not closed
	allIssues, response, err := a.Jira.Issue.Search(
		`Sprint IN openSprints()`,
		&jira.SearchOptions{
			StartAt:       0,
			MaxResults:    1000,
			ValidateQuery: "strict",
			Fields: []string{
				"customfield_10010", // Sprint
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
