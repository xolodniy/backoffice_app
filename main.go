package main

import (
	"fmt"
	"time"

	"backoffice_app/config"
	"backoffice_app/services"
)

var dateOfWorkdaysStart = time.Date(2018, 9, 10, 0, 0, 0, 0, time.Local)
var dateOfWorkdaysEnd = time.Date(2018, 9, 11, 23, 59, 59, 0, time.Local)

func main() {

	cfg := config.GetConfig()

	srvs, err := services.New(cfg)
	if err != nil {
		panic(err)
	}

	jiraAllIssues, _, err := srvs.Jira_IssuesSearch(cfg.Jira.IssueSearchParams)
	if err != nil {
		panic(err)
	}
	fmt.Printf("jiraAllIssues quantity: %v\n", len(jiraAllIssues))

	orgsList, err := srvs.Hubstaff_GetWorkersTimeByOrganization(dateOfWorkdaysStart, dateOfWorkdaysEnd, cfg.HubStaff.OrgsID)
	if err != nil {
		panic(fmt.Sprintf("HubStaff error: %v", err))
	}

	if len(orgsList) == 0 {
		err := srvs.Slack.SendStandardMessage("No tracked time for now or no organization found")
		if err != nil {
			panic(fmt.Sprintf("Slack error: can't decode response: %s", err))
		}
	}

	fmt.Printf("\nHubStaff output: %v\n", orgsList)

	var message = fmt.Sprintf(
		"Work time report\n\nFrom: %v %v\nTo: %v %v\n",
		dateOfWorkdaysStart.Format("02.01.06"), "00:00:00",
		dateOfWorkdaysEnd.Format("02.01.06"), "23:59:59",
	)
	for _, worker := range orgsList[0].Workers {
		message += fmt.Sprintf(
			"\n%s %s",
			secondsToClockTime(worker.TimeWorked),
			worker.Name,
		)
	}

	if len(orgsList[0].Workers) == 0 {
		message = "No tracked time for now or no workers found"
	}

	if err := srvs.Slack.SendStandardMessage(message); err != nil {
		panic(err)
	}
}
func secondsToClockTime(durationInSeconds int) string {
	workTime := time.Second * time.Duration(durationInSeconds)

	Hours := int(workTime.Hours())
	Minutes := int(workTime.Minutes())

	return fmt.Sprintf("%d%d:%d%d", Hours/10, Hours%10, Minutes/10, Minutes%10)
}
