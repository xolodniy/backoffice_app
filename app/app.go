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
			t, err := a.DurationStringInHoursMinutes(worker.TimeWorked)
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
			workerTime, err := a.DurationStringInHoursMinutes(worker.TimeWorked)
			if err != nil {
				logrus.WithError(err).WithField("time", worker.TimeWorked).
					Error("error occurred on worker's time conversion error")
				continue
			} else if worker.TimeWorked == 0 {
				continue
			}
			//employee name print
			message += fmt.Sprintf(
				"\n\n\n*%s (%s total)*\n", worker.Name, workerTime)
			for _, project := range worker.Projects {
				projectTime, err := a.DurationStringInHoursMinutes(project.TimeWorked)
				if err != nil {
					logrus.WithError(err).
						WithField("separatedDate", separatedDate.Date).
						WithField("worker", worker.Name).
						WithField("time", project.TimeWorked).
						Error("error occurred on projects's time conversion error")
					continue
				} else if project.TimeWorked == 0 {
					continue
				}
				message += fmt.Sprintf(
					"\n%s - %s", projectTime, project.Name)
				for _, note := range project.Notes {
					message += fmt.Sprintf("\n - %s", note.Description)
				}
			}
		}
	}
	a.Slack.SendMessage(message)
}

// DurationStringInHoursMinutes converts Seconds to 00:00 (hours with leading zero:minutes with leading zero) time format

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
		msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s>\n", issue.Key)
	}
	a.Slack.SendMessage(msgBody)
}

// ReportEmployeesWithExceededEstimateTime create report about employees with ETA overhead
func (a *App) ReportEmployeesWithExceededEstimateTime() {
	//getting actual sum of ETA from jira by employees
	remainingEtaMap := a.getActualRemainingEtaMap()
	if remainingEtaMap == nil {
		return
	}
	if len(remainingEtaMap) == 0 {
		a.Slack.SendMessage("There are no issues with remaining ETA.")
		return
	}
	//get logged time from Hubstaff for this week
	organizations, err := a.GetWorkersTimeByOrganization(now.BeginningOfWeek(), now.EndOfWeek(),
		a.Config.Hubstaff.OrgsID)
	if err != nil {
		logrus.WithError(err).Error("failed to fetch data from hubstaff")
		return
	}
	if len(organizations) == 0 || len(organizations[0].Workers) == 0 {
		a.Slack.SendMessage("No tracked time for now or no organization found")
		return
	}
	messageHeader := fmt.Sprintf("\nExceeded estimate time report:\n\n*%v*\n",
		now.BeginningOfDay().Format("02.01.2006"))
	message := compileMessageForExeededEmployeesReport(organizations, remainingEtaMap)
	a.Slack.SendMessage(fmt.Sprintf("%s\n%s", messageHeader, message))
}

func compileMessageForExeededEmployeesReport(organizations types.Organizations, remainingEtaMap map[string]int) string {
	message := ""
	var weekWorkingHours float32 = 30.0
	for _, worker := range organizations[0].Workers {
		etaTimeSec := remainingEtaMap[worker.Name]
		if etaTimeSec > 0 {
			workVolume := float32(etaTimeSec+worker.TimeWorked) / 3600.0
			if workVolume > weekWorkingHours {
				message += fmt.Sprintf("\n%s late for %.2f hours", worker.Name, workVolume-weekWorkingHours)
			}
		}
	}
	if message == "" {
		return "No one developer has exceeded estimate time"
	}
	return message
}

func (a *App) getActualRemainingEtaMap() map[string]int {
	remainingEtaMap := make(map[string]int)
	issues, err := a.Jira.AssigneeOpenIssues()
	if err != nil {
		logrus.WithError(err).Error("can't take information about closed subtasks from jira")
		return nil
	}
	for _, issue := range issues {
		if issue.Fields.Assignee == nil || issue.Fields.Assignee.DisplayName == "Unassigned" {
			continue
		}
		remainingEtaMap[issue.Fields.Assignee.DisplayName] += issue.Fields.TimeTracking.RemainingEstimateSeconds
	}
	return remainingEtaMap
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
		msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s>\n", issue.Key)
	}
	a.Slack.SendMessage(msgBody)
}
