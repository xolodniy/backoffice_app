package app

import (
	"fmt"
	"time"

	"backoffice_app/config"
	"backoffice_app/services/hubstaff"
	"backoffice_app/services/jira"
	"backoffice_app/services/slack"

	"github.com/jinzhu/now"
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

// MakeWorkersWorkedReportLastWeek preparing a last week report and send it to Slack
func (a *App) MakeWorkersWorkedReportLastWeek(mode string) {
	a.GetWorkersWorkedTimeAndSendToSlack(
		fmt.Sprintf("Weekly work time report (%s)", mode),
		now.BeginningOfWeek().AddDate(0, 0, -7),
		now.EndOfWeek().AddDate(0, 0, -7))
}

// MakeWorkersWorkedReportYesterday preparing a last day report and send it to Slack
func (a *App) MakeWorkersWorkedReportYesterday(mode string) {
	a.GetDetailedWorkersWorkedTimeAndSendToSlack(
		fmt.Sprintf("Daily detailed report (%s)", mode),
		now.BeginningOfDay().AddDate(0, 0, -1),
		now.EndOfDay().AddDate(0, 0, -1))
}

// GetWorkersWorkedTimeAndSendToSlack gather workers work time made through period between dates and send it to Slack channel
func (a *App) GetWorkersWorkedTimeAndSendToSlack(prefix string, dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time) {
	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")

	apiURL := fmt.Sprintf("/v1/custom/by_member/team/?start_date=%s&end_date=%s&organizations=%d",
		dateStart, dateEnd, a.Hubstaff.OrgID)
	orgsList, err := a.Hubstaff.RequestAndParse(apiURL)
	if err != nil {
		logrus.WithError(err).Error("can't get workers worked tim from Hubstaff")
		return
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
			message += fmt.Sprintf("\n%s %s", worker.TimeWorked, worker.Name)
		}
	}
	a.Slack.SendMessage(message)
}

// GetDetailedWorkersWorkedTimeAndSendToSlack gather detailed workers work time made through period between dates and send it to Slack channel
func (a *App) GetDetailedWorkersWorkedTimeAndSendToSlack(prefix string, dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time) {
	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")

	apiURL := fmt.Sprintf("/v1/custom/by_date/team/?start_date=%s&end_date=%s&organizations=%d&show_notes=%t",
		dateStart, dateEnd, a.Hubstaff.OrgID, true)
	orgsList, err := a.Hubstaff.RequestAndParse(apiURL)
	if err != nil {
		logrus.WithError(err).Error("can't get workers worked tim from Hubstaff")
		return
	}

	var message = prefix + "\n"

	if len(orgsList) == 0 {
		a.Slack.SendMessage("No tracked time for now or no organization found")
		return
	}

	if len(orgsList[0].Dates) == 0 {
		a.Slack.SendMessage("No tracked time for now or no workers found")
		return
	}
	for _, separatedDate := range orgsList[0].Dates {
		if separatedDate.TimeWorked == 0 {
			continue
		}
		//separatedDate print
		message += fmt.Sprintf(
			"\n\n\n*%s*", separatedDate.Date)
		for _, worker := range separatedDate.Workers {
			//employee name print
			message += fmt.Sprintf(
				"\n\n\n*%s (%s total)*\n", worker.Name, worker.TimeWorked)
			for _, project := range worker.Projects {
				message += fmt.Sprintf(
					"\n%s - %s", project.TimeWorked, project.Name)
				for _, note := range project.Notes {
					message += fmt.Sprintf("\n - %s", note.Description)
				}
			}
		}
	}
	a.Slack.SendMessage(message)
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
	size, err := a.Slack.FilesSize()
	if err != nil {
		logrus.WithError(err).Error("can't take information about files size from slack")
		return
	}
	if a.Slack.TotalVolume-size > a.Slack.RestVolume {
		return
	}
	msgBody := fmt.Sprintf("Free space on slack end.\n")
	a.Slack.SendMessage(msgBody)
}
