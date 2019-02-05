package app

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"backoffice_app/clients"
	"backoffice_app/config"

	"github.com/andygrunwald/go-jira"
	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

// App is main App implementation
type App struct {
	Hubstaff *clients.Hubstaff
	Slack    *clients.Slack
	Jira     *jira.Client
	Git      *gitlab.Client
	Config   config.Main
}

// New is main App constructor
func New(config *config.Main) (*App, error) {
	Hubstaff := &clients.Hubstaff{
		HTTPClient: http.DefaultClient,
		AppToken:   config.Hubstaff.Auth.AppToken,
		AuthToken:  config.Hubstaff.Auth.Token,
		APIURL:     config.Hubstaff.APIURL,
	}

	jiraClient, err := jira.NewClient(config.Jira.Auth.Client(), config.Jira.APIUrl)
	if err != nil {
		return nil, fmt.Errorf("Jira error: can't create jira client: %s", err)
	}

	slack := &clients.Slack{
		APIURL: config.Slack.APIURL,
		Auth: clients.SlackAuth{
			InToken:  config.Slack.Auth.InToken,
			OutToken: config.Slack.Auth.OutToken,
		},
		Channel: clients.SlackChannel{
			BotName: config.Slack.Channel.BotName,
			ID:      "#" + config.Slack.Channel.BackOfficeAppID,
		},
	}

	git := gitlab.NewClient(nil, config.GitToken)

	return &App{Hubstaff, slack, jiraClient, git, *config}, nil

}

// GetWorkersWorkedTimeAndSendToSlack gather workers work time made through period between dates and send it to Slack channel
func (a *App) GetWorkersWorkedTimeAndSendToSlack(prefix string, dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time, orgID int64) {
	orgsList, err := a.GetWorkersTimeByOrganization(dateOfWorkdaysStart, dateOfWorkdaysEnd, orgID)
	if err != nil {
		panic(fmt.Sprintf("Hubstaff error: %v", err))
	}

	var message = fmt.Sprintf(
		"%s:\n\n"+
			"From: %v %v\n"+
			"To: %v %v\n",
		prefix,
		dateOfWorkdaysStart.Format("02.01.06"), "00:00:00",
		dateOfWorkdaysEnd.Format("02.01.06"), "23:59:59",
	)

	if len(orgsList) == 0 {
		a.Slack.SendStandardMessage(
			"No tracked time for now or no organization found",
			a.Slack.Channel.ID,
			a.Slack.Channel.BotName,
		)
		return
	}

	if len(orgsList[0].Workers) == 0 {
		message = "No tracked time for now or no workers found"
	} else {
		for _, worker := range orgsList[0].Workers {
			t, err := a.DurationString(worker.TimeWorked)
			if err != nil {
				logrus.
					WithError(err).
					WithField("time", worker.TimeWorked).
					Error("error occurred on time conversion error")
				continue
			}
			message += fmt.Sprintf(
				"\n%s %s",
				t,
				worker.Name,
			)
		}
	}

	a.Slack.SendStandardMessage(
		message,
		a.Slack.Channel.ID,
		a.Slack.Channel.BotName,
	)

}

// DurationString converts Seconds to 00:00 (hours with leading zero:minutes with leading zero) time format
func (a *App) DurationString(durationInSeconds int) (string, error) {
	var someTime time.Time
	r, err := regexp.Compile(` ([0-9]{2,2}:[0-9]{2,2}):[0-9]{2,2}`)
	if err != nil {
		return "", fmt.Errorf("regexp error: %v", err)
	}

	occurrences := r.FindStringSubmatch(someTime.Add(time.Second * time.Duration(durationInSeconds)).String())
	if len(occurrences) != 2 && &occurrences[1] == nil {
		return "", fmt.Errorf("no time after unix time parsing")
	}

	return occurrences[1], nil
}

// ReportIsuuesWithClosedSubtasks create report about issues woth closed subtasks
func (a *App) ReportIsuuesWithClosedSubtasks() {
	allIssues, err := a.IssuesWithClosedSubtasks()
	if err != nil {
		panic(err)
	}
	var msgBody = "Issues have all closed subtasks:\n"
	for _, issue := range allIssues {
		msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s>: _%[1]s_\n",
			issue.Key,
		)
	}

	a.Slack.SendStandardMessage(
		msgBody,
		a.Slack.Channel.ID,
		a.Slack.Channel.BotName,
	)
}

// ReportEmployeesHaveExceededTasks create report about employees that have exceeded tasks
func (a *App) ReportEmployeesHaveExceededTasks() {
	allIssues, _, err := a.IssuesSearch()
	if err != nil {
		panic(err)
	}
	var index = 1
	var msgBody = "Employees have exceeded tasks:\n"
	for _, issue := range allIssues {
		if issue.Fields != nil {
			continue
		}
		if listRow := a.IssueTimeExceededNoTimeRange(issue, index); listRow != "" {
			msgBody += listRow
			index++
		}
	}

	a.Slack.SendStandardMessage(
		msgBody,
		a.Slack.Channel.ID,
		a.Slack.Channel.BotName,
	)
}
