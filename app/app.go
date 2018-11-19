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
)

type app struct {
	Hubstaff *clients.Hubstaff
	Slack    *clients.Slack
	Jira     *jira.Client
}

// New is main app constructor
func New(config *config.Main) (*app, error) {
	Hubstaff := &clients.Hubstaff{
		HTTPClient: http.DefaultClient,
		AppToken:   config.Hubstaff.Auth.AppToken,
		AuthToken:  config.Hubstaff.Auth.Token,
		APIUrl:     config.Hubstaff.APIUrl,
	}

	jiraClient, err := jira.NewClient(config.Jira.Auth.Client(), config.Jira.APIUrl)
	if err != nil {
		return nil, fmt.Errorf("Jira error: can't create jira client: %s", err)
	}

	slack := &clients.Slack{
		APIUrl: config.Slack.APIUrl,
		Auth: clients.SlackAuth{
			InToken:  config.Slack.Auth.InToken,
			OutToken: config.Slack.Auth.OutToken,
		},
		Channel: clients.SlackChannel{
			BotName: config.Slack.Channel.BotName,
			ID:      "#" + config.Slack.Channel.ID,
		},
	}

	return &app{Hubstaff, slack, jiraClient}, nil

}

// GetWorkersWorkedTimeAndSendToSlack gather workers work time made through period between dates and send it to Slack channel
func (a *app) GetWorkersWorkedTimeAndSendToSlack(prefix string, dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time, orgID int64) {
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
		err := a.Slack.SendStandardMessage(
			"No tracked time for now or no organization found",
			a.Slack.Channel.ID,
			a.Slack.Channel.BotName)
		if err != nil {
			panic(fmt.Sprintf("Slack error: %s", err))
		}
		return
	}

	//fmt.Println("Hubstaff output:")
	//spew.Dump(orgsList)

	// Only one organization needed for now
	if len(orgsList[0].Workers) == 0 {
		message = "No tracked time for now or no workers found"
	} else {
		for _, worker := range orgsList[0].Workers {
			t, err := a.SecondsToClockTime(worker.TimeWorked)
			if err != nil {
				logrus.Errorf("time conversion: regexp error: %v", err)
				continue
			}
			message += fmt.Sprintf(
				"\n%s %s",
				t,
				worker.Name,
			)
		}
	}

	if err := a.Slack.SendStandardMessage(message,
		a.Slack.Channel.ID,
		a.Slack.Channel.BotName); err != nil {
		panic(fmt.Sprintf("Slack error: %s", err))
	}

}

// SecondsToClockTime converts Seconds to 00:00 (hours with leading zero:minutes with leading zero) time format
func (_ *app) SecondsToClockTime(durationInSeconds int) (string, error) {
	var someTime time.Time
	r, err := regexp.Compile(` ([0-9]{2,2}:[0-9]{2,2}):[0-9]{2,2}`)
	if err != nil {
		return "", fmt.Errorf("time conversion: regexp error: %v", err)
	}

	occurrences := r.FindStringSubmatch(someTime.Add(time.Second * time.Duration(durationInSeconds)).String())
	if len(occurrences) != 2 && &occurrences[1] == nil {
		return "", fmt.Errorf("time conversion: no time after unix time parsing")
	}

	return occurrences[1], nil
}
