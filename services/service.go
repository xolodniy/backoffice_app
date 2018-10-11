package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"backoffice_app/clients"
	"backoffice_app/config"
	"backoffice_app/types"

	"github.com/andygrunwald/go-jira"
)

type service struct {
	HubStaff *clients.HubStaff
	Slack    *clients.Slack
	Jira     *jira.Client
}

func New(config config.Config) (*service, error) {
	hubstuff := &clients.HubStaff{
		HTTPClient: http.DefaultClient,
		AppToken:   config.HubStaff.Auth.AppToken,
		APIUrl:     config.HubStaff.APIUrl,
	}

	if err := hubstuff.Authorize(config.HubStaff.Auth); err != nil {
		return nil, err
	}

	jira, err := jira.NewClient(config.Jira.Auth.Client(), config.Jira.APIUrl)
	if err != nil {
		return nil, fmt.Errorf("can't create jira client: %s", err)
	}

	slack := &clients.Slack{
		Auth: types.SlackAuth{
			InToken:  config.Slack.Auth.InToken,
			OutToken: config.Slack.Auth.OutToken,
		},
		Channel: types.SlackChannel{
			BotName: config.Slack.Channel.BotName,
			ID:      config.Slack.Channel.ID,
		},
	}

	return &service{hubstuff, slack, jira}, nil

}

func (c *service) Hubstaff_GetWorkersTimeByOrganization(dateOfWorkdaysStart time.Time, dateOfWorkdaysEnd time.Time, OrgID int64) (types.Organizations, error) {

	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")

	orgsRaw, err := c.HubStaff.Request(
		fmt.Sprintf(
			"/v1/custom/by_member/team/?start_date=%s&end_date=%s&organizations=%d",
			dateStart,
			dateEnd,
			OrgID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("error on getting workers worked time: %v", err)
	}

	orgs := struct {
		List types.Organizations `json:"organizations"`
	}{}

	if err = json.Unmarshal(orgsRaw, &orgs); err != nil {
		return nil, fmt.Errorf("can't decode response: %s", err)
	}
	return orgs.List, nil
}

func (c *service) Jira_IssuesSearch(searchParams types.JiraIssueSearchParams) ([]jira.Issue, *jira.Response, error) {
	// allIssues including issues from other sprints and not closed
	var _, _ = searchParams.JQL, &searchParams.Options
	allIssues, response, err := c.Jira.Issue.Search(
		searchParams.JQL,
		searchParams.Options,
	)

	if err != nil {
		return nil, response, fmt.Errorf("can't create jira client: %s", err)
	}

	return allIssues, response, nil
}
