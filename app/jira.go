package app

import (
	"fmt"

	"backoffice_app/types"

	"github.com/andygrunwald/go-jira"
)

func (a *app) IssuesSearch(searchParams types.JiraIssueSearchParams) ([]jira.Issue, *jira.Response, error) {
	// allIssues including issues from other sprints and not closed
	var _, _ = searchParams.JQL, &searchParams.Options
	allIssues, response, err := a.Jira.Issue.Search(
		`project = "CRA" AND Sprint IN openSprints()`,
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
