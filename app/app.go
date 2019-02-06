package app

import (
	"fmt"
	"regexp"
	"time"

	"backoffice_app/config"
	"backoffice_app/services/hubstaff"
	"backoffice_app/services/jiraloc"
	"backoffice_app/services/slack"

	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

// App is main App implementation
type App struct {
	Hubstaff hubstaff.Hubstaff
	Slack    slack.Slack
	Jira     jiraloc.Jira
	Git      *gitlab.Client
	Config   config.Main
}

// New is main App constructor
func New(conf *config.Main) (*App, error) {
	git := gitlab.NewClient(nil, conf.GitToken)

	return &App{
		Hubstaff: hubstaff.New(&conf.Hubstaff),
		Slack:    slack.New(&conf.Slack),
		Jira:     jiraloc.New(&conf.Jira),
		Git:      git,
		Config:   *conf,
	}, nil

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
		err := a.Slack.SendMessage(
			"No tracked time for now or no organization found",
			a.Slack.ID,
			a.Slack.BotName,
			false,
			"",
		)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"msgBody":        "No tracked time for now or no organization found",
				"channelID":      a.Slack.ID,
				"channelBotName": a.Slack.BotName,
			}).Error(err.Error())
		}
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

	err = a.Slack.SendMessage(
		message,
		a.Slack.ID,
		a.Slack.BotName,
		false,
		"",
	)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        message,
			"channelID":      a.Slack.ID,
			"channelBotName": a.Slack.BotName,
		}).Error(err.Error())
	}

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
	allIssues, err := a.Jira.IssuesWithClosedSubtasks()
	if err != nil {
		panic(err)
	}
	var msgBody = "Issues have all closed subtasks:\n"
	for _, issue := range allIssues {
		msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s>: _%[1]s_\n",
			issue.Key,
		)
	}

	err = a.Slack.SendMessage(
		msgBody,
		a.Slack.ID,
		a.Slack.BotName,
		false,
		"",
	)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        msgBody,
			"channelID":      a.Slack.ID,
			"channelBotName": a.Slack.BotName,
		}).Error(err.Error())
	}
}

// ReportEmployeesHaveExceededTasks create report about employees that have exceeded tasks
func (a *App) ReportEmployeesHaveExceededTasks() {
	allIssues, err := a.Jira.AssigneeOpenIssues()
	if err != nil {
		panic(err)
	}
	var index = 1
	var msgBody = "Employees have exceeded tasks:\n"
	for _, issue := range allIssues {
		if issue.Fields != nil {
			continue
		}
		if listRow := a.Jira.IssueTimeExceededNoTimeRange(issue, index); listRow != "" {
			msgBody += listRow
			index++
		}
	}

	err = a.Slack.SendMessage(
		msgBody,
		a.Slack.ID,
		a.Slack.BotName,
		false,
		"",
	)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        msgBody,
			"channelID":      a.Slack.ID,
			"channelBotName": a.Slack.BotName,
		}).Error(err.Error())
	}
}
