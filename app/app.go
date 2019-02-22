package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"backoffice_app/config"
	"backoffice_app/services/bitbucket"
	"backoffice_app/services/hubstaff"
	"backoffice_app/services/jira"
	"backoffice_app/services/slack"

	"github.com/jinzhu/now"
	"github.com/sirupsen/logrus"
)

// CommitsCache struct of hash commits map
type CommitsCache struct {
	Repository string
	Path       string
	Message    string
}

// App is main App implementation
type App struct {
	Hubstaff     hubstaff.Hubstaff
	Slack        slack.Slack
	Jira         jira.Jira
	Bitbucket    bitbucket.Bitbucket
	Config       config.Main
	CommitsCache map[string]CommitsCache
}

// New is main App constructor
func New(conf *config.Main) *App {
	return &App{
		Hubstaff:     hubstaff.New(&conf.Hubstaff),
		Slack:        slack.New(&conf.Slack),
		Jira:         jira.New(&conf.Jira),
		Bitbucket:    bitbucket.New(&conf.Bitbucket),
		Config:       *conf,
		CommitsCache: make(map[string]CommitsCache),
	}
}

// MakeWorkersWorkedReportLastWeek preparing a last week report and send it to Slack
func (a *App) MakeWorkersWorkedReportLastWeek(mode string) {
	a.ReportWorkersWorkedTime(
		fmt.Sprintf("Weekly work time report (%s)", mode),
		now.BeginningOfWeek().AddDate(0, 0, -7),
		now.EndOfWeek().AddDate(0, 0, -7))
}

// MakeWorkersWorkedReportYesterday preparing a last day report and send it to Slack
func (a *App) MakeWorkersWorkedReportYesterday(mode string) {
	a.ReportWorkersWorkedTimeDetailed(
		fmt.Sprintf("Daily detailed report (%s)", mode),
		now.BeginningOfDay().AddDate(0, 0, -1),
		now.EndOfDay().AddDate(0, 0, -1))
}

// ReportWorkersWorkedTime gather workers work time made through period between dates and send it to Slack channel
func (a *App) ReportWorkersWorkedTime(prefix string, dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time) {
	hubstaffResponse, err := a.Hubstaff.WorkersWorkedTimeDetailed(dateOfWorkdaysStart, dateOfWorkdaysEnd)
	if err != nil {
		logrus.WithError(err).Error("can't get workers worked time from Hubstaff")
		return
	}
	var message = fmt.Sprintf(
		"%s:\n\nFrom: %v %v\nTo: %v %v\n", prefix,
		dateOfWorkdaysStart.Format("02.01.06"), "00:00:00",
		dateOfWorkdaysEnd.Format("02.01.06"), "23:59:59",
	)
	var workers = make(map[string]int)
	for _, separatedDate := range hubstaffResponse.Dates {
		if separatedDate.Duration == 0 {
			continue
		}
		for _, worker := range separatedDate.Workers {
			workers[worker.Name] += worker.Duration
		}
	}
	for name, duration := range workers {
		//employee name print
		message += fmt.Sprintf("%s (%s total)\n", name, a.TimeWorkedConverter(duration))
	}
	a.Slack.SendMessage(message, a.Slack.ChanBackofficeApp)
}

// ReportWorkersWorkedTimeDetailed gather detailed workers work time made through period between dates and send it to Slack channel
func (a *App) ReportWorkersWorkedTimeDetailed(prefix string, dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time) {
	hubstaffResponse, err := a.Hubstaff.WorkersWorkedTimeDetailed(dateOfWorkdaysStart, dateOfWorkdaysEnd)
	if err != nil {
		logrus.WithError(err).Error("can't get detailed workers worked time from Hubstaff")
		return
	}
	var message = prefix + "\n"
	for _, separatedDate := range hubstaffResponse.Dates {
		if separatedDate.Duration == 0 {
			continue
		}
		//separatedDate print
		message += fmt.Sprintf("\n\n\n*%s*", separatedDate.Date)
		for _, worker := range separatedDate.Workers {
			//employee name print
			message += fmt.Sprintf("\n\n\n*%s (%s total)*\n", worker.Name, a.TimeWorkedConverter(worker.Duration))
			for _, project := range worker.Projects {
				message += fmt.Sprintf("\n%s - %s", a.TimeWorkedConverter(project.Duration), project.Name)
				for _, note := range project.Notes {
					message += fmt.Sprintf("\n - %s", note.Description)
				}
			}
		}
	}
	a.Slack.SendMessage(message, a.Slack.ChanBackofficeApp)
}

// ReportIsuuesWithClosedSubtasks create report about issues with closed subtasks
func (a *App) ReportIsuuesWithClosedSubtasks() {
	issues, err := a.Jira.IssuesWithClosedSubtasks()
	if err != nil {
		logrus.WithError(err).Error("can't take information about closed subtasks from jira")
		return
	}
	if len(issues) == 0 {
		a.Slack.SendMessage("There are no issues with all closed subtasks", a.Slack.ChanBackofficeApp)
		return
	}
	msgBody := "Issues have all closed subtasks:\n"
	for _, issue := range issues {
		msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_\n",
			issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
	}
	a.Slack.SendMessage(msgBody, a.Slack.ChanBackofficeApp)
}

// ReportEmployeesWithExceededEstimateTime create report about employees with ETA overhead
func (a *App) ReportEmployeesWithExceededEstimateTime() {
	//getting actual sum of ETA from jira by employees
	jiraRemainingEtaMap := make(map[string]int)
	issues, err := a.Jira.AssigneeOpenIssues()
	if err != nil {
		logrus.WithError(err).Error("can't take information about closed subtasks from jira")
		return
	}
	for _, issue := range issues {
		jiraRemainingEtaMap[issue.Fields.Assignee.EmailAddress] += issue.Fields.TimeTracking.RemainingEstimateSeconds
	}
	if len(jiraRemainingEtaMap) == 0 {
		a.Slack.SendMessage("There are no issues with remaining ETA.", a.Slack.ChanBackofficeApp)
		return
	}
	hubstaffResponse, err := a.Hubstaff.WorkersWorkedTimeDetailed(now.BeginningOfWeek(), now.EndOfWeek())
	if err != nil {
		logrus.WithError(err).Error("can't get logged time from Hubstaff")
		return
	}
	//get hubstaff's user list
	response, err := a.Hubstaff.Users()
	if err != nil {
		logrus.WithError(err).Error("failed to fetch data from hubstaff")
		return
	}
	// prepare the content
	messageHeader := fmt.Sprintf("\nExceeded estimate time report:\n\n*%v*\n", now.BeginningOfDay().Format("02.01.2006"))
	message := ""
	var workers = make(map[string]int)
	for _, separatedDate := range hubstaffResponse.Dates {
		if separatedDate.Duration == 0 {
			continue
		}
		for _, worker := range separatedDate.Workers {
			workers[worker.Name] += worker.Duration
		}
	}
	for _, user := range response.Users {
		if jiraRemainingEtaMap[user.Email] > 0 {
			workVolume := float32(jiraRemainingEtaMap[user.Email]+workers[user.Name]) / 3600.0
			if workVolume > a.Config.MaxWeekWorkingHours {
				message += fmt.Sprintf("\n%s late for %.2f hours", user.Name, workVolume-a.Config.MaxWeekWorkingHours)
			}
		}
	}
	if message == "" {
		message = "No one developer has exceeded estimate time"
	}
	a.Slack.SendMessage(fmt.Sprintf("%s\n%s", messageHeader, message), a.Slack.ChanBackofficeApp)
}

// ReportEmployeesHaveExceededTasks create report about employees that have exceeded tasks
func (a *App) ReportEmployeesHaveExceededTasks() {
	issues, err := a.Jira.AssigneeOpenIssues()
	if err != nil {
		logrus.WithError(err).Error("can't take information about exceeded tasks of employees from jira")
		return
	}
	if len(issues) == 0 {
		a.Slack.SendMessage("There are no employees with exceeded subtasks", a.Slack.ChanBackofficeApp)
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
	a.Slack.SendMessage(msgBody, a.Slack.ChanBackofficeApp)
}

// ReportIsuuesAfterSecondReview create report about issues after second review round
func (a *App) ReportIsuuesAfterSecondReview() {
	issues, err := a.Jira.IssuesAfterSecondReview()
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues after second review from jira")
		return
	}
	if len(issues) == 0 {
		a.Slack.SendMessage("There are no issues after second review round", a.Slack.ChanBackofficeApp)
		return
	}
	msgBody := "Issues after second review round:\n"
	for _, issue := range issues {
		msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_\n",
			issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
	}
	a.Slack.SendMessage(msgBody, a.Slack.ChanBackofficeApp)
}

// ReportSlackEndingFreeSpace create report about employees that have exceeded tasks
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
	a.Slack.SendMessage(msgBody, a.Slack.ChanBackofficeApp)
}

// ReportGitMigrations create report about new git migrations
func (a *App) ReportGitMigrations() {
	messages, err := a.MigrationMessages()
	if err != nil {
		logrus.WithError(err).Error("can't take information git migrations from bitbucket")
		return
	}
	for _, message := range messages {
		if message != "" {
			a.Slack.SendMessage(message, a.Slack.ChanMigrations)
		}
	}
}

// FillCache fill cache commits for searching new migrations
func (a *App) FillCache() {
	commits, err := a.Bitbucket.CommitsOfOpenedPRs()
	if err != nil {
		logrus.WithError(err).Error("can't take information about opened commits from bitbucket")
		return
	}
	mapSQLCommits, err := a.SQLCommitsCache(commits)
	if err != nil {
		logrus.WithError(err).Error("can't take diff information from bitbucket")
		return
	}
	a.CommitsCache = mapSQLCommits
}

// MigrationMessages returns slice of all miigration files
func (a *App) MigrationMessages() ([]string, error) {
	commits, err := a.Bitbucket.CommitsOfOpenedPRs()
	if err != nil {
		logrus.WithError(err).Error("can't take information about opened commits from bitbucket")
		return []string{}, err
	}

	newCommitsCache, err := a.SQLCommitsCache(commits)
	if err != nil {
		logrus.WithError(err).Error("can't take diff information from bitbucket")
		return nil, err
	}
	var files []string
	for hash, cache := range newCommitsCache {
		if _, ok := a.CommitsCache[hash]; !ok {
			file, err := a.Bitbucket.SrcFile(cache.Repository, hash, cache.Path)
			if err != nil {
				logrus.WithError(err).Error("can't take information about file from bitbucket")
				return []string{}, err
			}
			files = append(files, cache.Message+"\n```"+file+"```\n")
		}
	}
	a.CommitsCache = newCommitsCache
	return files, nil
}

// SQLCommitsCache returns commits cache with sql migration
func (a *App) SQLCommitsCache(commits []bitbucket.Commit) (map[string]CommitsCache, error) {
	newMapSQLCommits := make(map[string]CommitsCache)
	for _, commit := range commits {
		diffStats, err := a.Bitbucket.CommitsDiffStats(commit.Repository.Name, commit.Hash)
		if err != nil {
			return nil, err
		}
		for _, diffStat := range diffStats {
			if strings.Contains(diffStat.New.Path, ".sql") {
				newMapSQLCommits[commit.Hash] = CommitsCache{Repository: commit.Repository.Name, Path: diffStat.New.Path, Message: commit.Message}
			}
		}
	}
	return newMapSQLCommits, nil
}

// ReportLastActivityCallback posts last activity to slack user by callbackUrl
func (a *App) ReportLastActivityCallback(callbackURL string) {
	response, err := a.Hubstaff.LastActivities()
	if err != nil {
		logrus.WithError(err).Error("Can't get last activity report from Hubstaff.")
		return
	}
	if len(response.LastActivities) == 0 {
		a.Slack.SendMessage("No logged activities have found", a.Slack.ChanBackofficeApp)
		return
	}

	message := ""
	for _, activity := range response.LastActivities {
		projectName, err := a.Hubstaff.ProjectName(activity.LastProjectID)
		if err != nil {
			continue
		}
		response, err := a.Hubstaff.JiraTask(activity.LastTaskID)
		if err != nil {
			logrus.WithError(err).Error("Can't get jira task from Hubstaff.")
			return
		}
		if projectName != "" || response.Task.JiraKey != "" {
			message += fmt.Sprintf("\n\n*%s*\n%s", activity.User.Name, projectName)
			if response.Task.JiraKey != "" {
				message += fmt.Sprintf(" <https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>",
					response.Task.JiraKey, response.Task.Summary)
			}
		}
	}
	jsonReport, err := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: message})
	if err != nil {
		logrus.WithError(err).Errorf("Can't convert last activity report to json. Report is:\n%s", message)
		return
	}
	resp, err := http.Post(callbackURL, "application/json", bytes.NewReader(jsonReport))
	if err != nil {
		logrus.WithError(err).Errorf("Can't send last activity report by url : %s", callbackURL)
		return
	}
	if resp.StatusCode != http.StatusOK {
		logrus.Errorf("Error while sending last activity report by url : %s, status code : %d",
			callbackURL, resp.StatusCode)
	}
}

// ReportLastActivity sends report of last activity to back office channel
func (a *App) ReportLastActivity() {
	response, err := a.Hubstaff.LastActivities()
	if err != nil {
		logrus.WithError(err).Error("Can't get last activity report from Hubstaff.")
		return
	}
	if len(response.LastActivities) == 0 {
		a.Slack.SendMessage("No logged activities have found", a.Slack.ChanBackofficeApp)
		return
	}

	message := ""
	for _, activity := range response.LastActivities {
		projectName, err := a.Hubstaff.ProjectName(activity.LastProjectID)
		if err != nil {
			continue
		}
		response, err := a.Hubstaff.JiraTask(activity.LastTaskID)
		if err != nil {
			a.Slack.SendMessage("Can't take last activity from hubstaff", a.Slack.ChanBackofficeApp)
			return
		}
		if projectName != "" || response.Task.JiraKey != "" {
			message += fmt.Sprintf("\n\n*%s*\n%s", activity.User.Name, projectName)
			if response.Task.JiraKey != "" {
				message += fmt.Sprintf(" <https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>",
					response.Task.JiraKey, response.Task.Summary)
			}
		}
	}
	a.Slack.SendMessage(message, a.Slack.ChanBackofficeApp)
}

// TimeWorkedConverter converts seconds value to 00:00 (hours with leading zero:minutes with leading zero) time format
func (a *App) TimeWorkedConverter(dur int) string {
	secInMin := 60
	secInHour := 60 * secInMin
	hours := int(dur) / secInHour
	minutes := int(dur) % secInHour / secInMin
	return fmt.Sprintf("%.2d:%.2d", hours, minutes)
}
