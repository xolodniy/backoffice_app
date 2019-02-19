package app

import (
	"fmt"
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
	hubstaffResponse, err := a.Hubstaff.RequestAndParseTimelogs(apiURL)
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
	for _, worker := range hubstaffResponse.Workers {
		message += fmt.Sprintf("\n%s %s", worker.TimeWorked, worker.Name)
	}
	a.Slack.SendMessage(message, a.Slack.ChanBackofficeApp)
}

// GetDetailedWorkersWorkedTimeAndSendToSlack gather detailed workers work time made through period between dates and send it to Slack channel
func (a *App) GetDetailedWorkersWorkedTimeAndSendToSlack(prefix string, dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time) {
	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")

	apiURL := fmt.Sprintf("/v1/custom/by_date/team/?start_date=%s&end_date=%s&organizations=%d&show_notes=%t",
		dateStart, dateEnd, a.Hubstaff.OrgID, true)
	hubstaffResponse, err := a.Hubstaff.RequestAndParseTimelogs(apiURL)
	if err != nil {
		logrus.WithError(err).Error("can't get workers worked tim from Hubstaff")
		return
	}

	var message = prefix + "\n"

	for _, separatedDate := range hubstaffResponse.Dates {
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
		if issue.Fields.Assignee == nil || issue.Fields.Assignee.DisplayName == "Unassigned" {
			continue
		}
		jiraRemainingEtaMap[issue.Fields.Assignee.EmailAddress] += issue.Fields.TimeTracking.RemainingEstimateSeconds
	}
	if len(jiraRemainingEtaMap) == 0 {
		a.Slack.SendMessage("There are no issues with remaining ETA.", a.Slack.ChanBackofficeApp)
		return
	}
	//get logged time from Hubstaff for this week
	var dateStart = now.BeginningOfWeek().Format("2006-01-02")
	var dateEnd = now.EndOfWeek().Format("2006-01-02")

	apiURL := fmt.Sprintf("/v1/custom/by_member/team/?start_date=%s&end_date=%s&organizations=%d",
		dateStart, dateEnd, a.Hubstaff.OrgID)
	hubstaffResponse, err := a.Hubstaff.RequestAndParseTimelogs(apiURL)
	if err != nil {
		logrus.WithError(err).Error("can't get logged time from Hubstaff")
		return
	}
	//get hubstaff's user list
	hubstaffUsers, err := a.Hubstaff.GetAllHubstaffUsers()
	if err != nil {
		logrus.WithError(err).Error("failed to fetch data from hubstaff")
		return
	}
	// prepare the content
	messageHeader := fmt.Sprintf("\nExceeded estimate time report:\n\n*%v*\n",
		now.BeginningOfDay().Format("02.01.2006"))
	message := ""
	var maxWeekWorkingHours float32 = 30.0
	for _, userWithTime := range hubstaffResponse.Workers {
		var workerEmail = ""
		for _, userWithEmail := range hubstaffUsers {
			if userWithEmail.Name == userWithTime.Name {
				workerEmail = userWithEmail.Email
				break
			}
		}
		jiraEta := jiraRemainingEtaMap[workerEmail]
		if jiraEta > 0 {
			workVolume := float32(jiraEta+int(userWithTime.TimeWorked)) / 3600.0
			if workVolume > maxWeekWorkingHours {
				message += fmt.Sprintf("\n%s late for %.2f hours", userWithTime.Name, workVolume-maxWeekWorkingHours)
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
	mapSqlCommits, err := a.SqlCommitsCache(commits)
	if err != nil {
		logrus.WithError(err).Error("can't take diff information from bitbucket")
		return
	}
	a.CommitsCache = mapSqlCommits
}

// MigrationMessages returns slice of all miigration files
func (a *App) MigrationMessages() ([]string, error) {
	commits, err := a.Bitbucket.CommitsOfOpenedPRs()
	if err != nil {
		logrus.WithError(err).Error("can't take information about opened commits from bitbucket")
		return []string{}, err
	}

	newCommitsCache, err := a.SqlCommitsCache(commits)
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

// SqlCommits returns commits cache with sql migration
func (a *App) SqlCommitsCache(commits []bitbucket.Commit) (map[string]CommitsCache, error) {
	newMapSqlCommits := make(map[string]CommitsCache)
	for _, commit := range commits {
		diffStats, err := a.Bitbucket.CommitsDiffStats(commit.Repository.Name, commit.Hash)
		if err != nil {
			return nil, err
		}
		for _, diffStat := range diffStats {
			if strings.Contains(diffStat.New.Path, ".sql") {
				newMapSqlCommits[commit.Hash] = CommitsCache{Repository: commit.Repository.Name, Path: diffStat.New.Path, Message: commit.Message}
			}
		}
	}
	return newMapSqlCommits, nil
}

// ReportGitMigrations create report about new git migrations
func (a *App) ReportSprintStatus() {
	var developers = make(map[string][]jira.Issue)
	issues, err := a.Jira.IssuesOfOpenSprints()
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues of open sprint from jira")
		return
	}
	if len(issues) == 0 {
		a.Slack.SendMessage("There are no issues of open sprint", a.Slack.ChanBackofficeApp)
		return
	}
	msgBody := "Sprint status:\n"
	for _, issue := range issues {
		developer := "No developer"
		developerMap, err := issue.Fields.Unknowns.MarshalMap("customfield_10026")
		if err != nil {
			logrus.WithError(err).WithField("developerMap", fmt.Sprintf("%+v", developerMap)).
				Error("can't make customfield_10026 map marshaling")
		}
		if developerMap != nil {
			displayName, ok := developerMap["displayName"].(string)
			if !ok {
				logrus.WithField("displayName", fmt.Sprintf("%+v", developerMap["displayName"])).
					Error("can't assert to string map displayName field")
			}
			developer = displayName
		}
		developers[developer] = append(developers[developer], issue)
	}
	for developer, issues := range developers {
		var message string
		for _, issue := range issues {
			if issue.Fields.Status.Name != jira.StatusClosed && issue.Fields.Status.Name != jira.StatusInClarification {
				message += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_\n",
					issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
			}
		}
		if message == "" {
			msgBody += fmt.Sprintf(developer + " - all tasks closed.\n")
			continue
		}
		msgBody += fmt.Sprintf(developer + " - has open tasks:\n" + message)
	}
	a.Slack.SendMessage(msgBody, a.Slack.ChanBackofficeApp)
}
