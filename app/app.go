package app

import (
	"fmt"
	"net/http"
	"time"

	"backoffice_app/clients"
	"backoffice_app/config"

	"github.com/andygrunwald/go-jira"
	"github.com/davecgh/go-spew/spew"
)

type app struct {
	Hubstaff *clients.Hubstaff
	Slack    *clients.Slack
	Jira     *jira.Client
}

func New(config *config.Main) (*app, error) {
	Hubstaff := &clients.Hubstaff{
		HTTPClient: http.DefaultClient,
		AppToken:   config.Hubstaff.Auth.AppToken,
		APIUrl:     config.Hubstaff.APIUrl,
	}
	Hubstaff.SetAuthToken(config.Hubstaff.Auth.Token)

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

func (a *app) GetWorkersWorkedTimeAndSendToSlack(dateOfWorkdaysStart time.Time, dateOfWorkdaysEnd time.Time, orgID int64) {
	orgsList, err := a.GetWorkersTimeByOrganization(dateOfWorkdaysStart, dateOfWorkdaysEnd, orgID)
	if err != nil {
		panic(fmt.Sprintf("Hubstaff error: %v", err))
	}

	var message = fmt.Sprintf(
		"Work time report\n\n"+
			"From: %v %v\n"+
			"To: %v %v\n",
		dateOfWorkdaysStart.Format("02.01.06"), "00:00:00",
		dateOfWorkdaysEnd.Format("02.01.06"), "23:59:59",
	)

	if len(orgsList) == 0 {
		err := a.Slack.SendStandardMessage(
			"No tracked time for now or no organization found",
			a.Slack.Channel.ID,
			a.Slack.Channel.BotName)
		if err != nil {
			panic(fmt.Sprintf("Slack error: %a", err))
		}
		return
	}

	fmt.Println("Hubstaff output:")
	spew.Dump(orgsList)

	// Only one organization needed for now
	if len(orgsList[0].Workers) == 0 {
		message = "No tracked time for now or no workers found"
	} else {
		for _, worker := range orgsList[0].Workers {
			message += fmt.Sprintf(
				"\n%a %a",
				secondsToClockTime(worker.TimeWorked),
				worker.Name,
			)
		}
	}

	if err := a.Slack.SendStandardMessage(message,
		a.Slack.Channel.ID,
		a.Slack.Channel.BotName); err != nil {
		panic(fmt.Sprintf("Slack error: %a", err))
	}

}

func secondsToClockTime(durationInSeconds int) string {
	workTime := time.Second * time.Duration(durationInSeconds)

	Hours := int(workTime.Hours())
	Minutes := int(workTime.Minutes())

	return fmt.Sprintf("%d%d:%d%d", Hours/10, Hours%10, Minutes/10, Minutes%10)
}
