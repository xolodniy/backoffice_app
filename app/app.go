package app

import (
	"fmt"
	"regexp"
	"time"

	"backoffice_app/config"
	"backoffice_app/services/hubstaff"
	"backoffice_app/services/jira"
	"backoffice_app/services/slack"

	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

// App is main App implementation
type App struct {
	Hubstaff hubstaff.Hubstaff
	Slack    slack.Slack
	Jira     jira.Jira
	Git      *gitlab.Client
	Config   config.Main
}

var slackMinSize = 0.5

// New is main App constructor
func New(conf *config.Main) *App {
	return &App{
		Hubstaff: hubstaff.New(&conf.Hubstaff),
		Slack:    slack.New(&conf.Slack),
		Jira:     jira.New(&conf.Jira),
		Git:      gitlab.NewClient(nil, conf.GitToken),
		Config:   *conf,
	}
}

// GetWorkersWorkedTimeAndSendToSlack gather workers work time made through period between dates and send it to Slack channel
func (a *App) GetWorkersWorkedTimeAndSendToSlack(prefix string, dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time, orgID int64) {
	orgsList, err := a.GetWorkersTimeByOrganization(dateOfWorkdaysStart, dateOfWorkdaysEnd, orgID)
	if err != nil {
		logrus.WithError(err).Error("can't get workers worked tim from Hubstaff")
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
		a.Slack.SendMessage("No tracked time for now or no organization found")
		return
	}

	if len(orgsList[0].Workers) == 0 {
		message = "No tracked time for now or no workers found"
	} else {
		for _, worker := range orgsList[0].Workers {
			t, err := a.DurationString(worker.TimeWorked)
			if err != nil {
				logrus.WithError(err).WithField("time", worker.TimeWorked).
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

	a.Slack.SendMessage(message)
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

// ReportIsuuesWithClosedSubtasks create report about issues with closed subtasks
func (a *App) ReportIsuuesWithClosedSubtasks() {
	issues, err := a.Jira.IssuesWithClosedSubtasks()
	if err != nil {
		logrus.WithError(err).Error("can't take information about closed subtasks from jira")
		return
	}
	if len(issues) == 0 {
		a.Slack.SendMessage("There are no issues with all closed subtasks")
		return
	}
	msgBody := "Issues have all closed subtasks:\n"
	for _, issue := range issues {
		msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_\n",
			issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
	}
	a.Slack.SendMessage(msgBody)
}

// ReportEmployeesHaveExceededTasks create report about employees that have exceeded tasks
func (a *App) ReportEmployeesHaveExceededTasks() {
	issues, err := a.Jira.AssigneeOpenIssues()
	if err != nil {
		logrus.WithError(err).Error("can't take information about exceeded tasks of employees from jira")
		return
	}
	if len(issues) == 0 {
		a.Slack.SendMessage("There are no employees with exceeded subtasks")
		return
	}
	var index = 1
	msgBody := "Employees have exceeded tasks:\n"
	for _, issue := range issues {
		if issue.Fields == nil {
			continue
		}
		if listRow := a.Jira.IssueTimeExceededNoTimeRange(issue, index); listRow != "" {
			msgBody += listRow
			index++
		}
	}
	a.Slack.SendMessage(msgBody)
}

// ReportIsuuesAfterSecondReview create report about issues after second review round
func (a *App) ReportIsuuesAfterSecondReview() {
	issues, err := a.Jira.IssuesAfterSecondReview()
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues after second review from jira")
		return
	}
	if len(issues) == 0 {
		a.Slack.SendMessage("There are no issues after second review round")
		return
	}
	msgBody := "Issues after second review round:\n"
	for _, issue := range issues {
		msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_\n",
			issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
	}
	a.Slack.SendMessage(msgBody)
}

// ReportEmployeesHaveExceededTasks create report about employees that have exceeded tasks
func (a *App) ReportSlackEndingFreeSpace() {
	free, err := a.Slack.FreeSpace()
	if err != nil {
		logrus.WithError(err).Error("can't take information about files size from slack")
		return
	}
	if free > slackMinSize {
		return
	}
	msgBody := fmt.Sprintf("Free space on slack is ending: %f Gb\n", free)
	a.Slack.SendMessage(msgBody)
}
