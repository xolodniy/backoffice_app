package app

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
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
func (a *App) MakeWorkersWorkedReportLastWeek(mode, channel string) {
	a.ReportUsersWorkedTimeByMember(
		fmt.Sprintf("Weekly work time report (%s)", mode), channel,
		now.BeginningOfWeek().AddDate(0, 0, -7),
		now.EndOfWeek().AddDate(0, 0, -7))
}

// MakeWorkersWorkedReportYesterday preparing a last day report and send it to Slack
func (a *App) MakeWorkersWorkedReportYesterday(mode, channel string) {
	a.ReportUsersWorkedTimeByDate(
		fmt.Sprintf("Daily detailed report (%s)", mode), channel,
		now.BeginningOfDay().AddDate(0, 0, -1),
		now.EndOfDay().AddDate(0, 0, -1))
}

// ReportUsersWorkedTimeByMember gather workers work time made through period between dates and send it to Slack channel
func (a *App) ReportUsersWorkedTimeByMember(prefix, channel string, dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time) {
	usersReports, err := a.Hubstaff.UsersWorkTimeByMember(dateOfWorkdaysStart, dateOfWorkdaysEnd)
	if err != nil {
		logrus.WithError(err).Error("can't get workers worked time by member from Hubstaff")
		return
	}
	var message = fmt.Sprintf(
		"%s:\n\nFrom: %v %v\nTo: %v %v\n", prefix,
		dateOfWorkdaysStart.Format("02.01.06"), "00:00:00",
		dateOfWorkdaysEnd.Format("02.01.06"), "23:59:59",
	)
	for _, user := range usersReports {
		message += fmt.Sprintf("\n%s %s", user.TimeWorked, user.Name)
	}
	a.Slack.SendMessage(message, channel)
}

// ReportUsersWorkedTimeByDate gather detailed workers work time made through period between dates and send it to Slack channel
func (a *App) ReportUsersWorkedTimeByDate(prefix, channel string, dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time) {
	datesReports, err := a.Hubstaff.UsersWorkTimeByDate(dateOfWorkdaysStart, dateOfWorkdaysEnd)
	if err != nil {
		logrus.WithError(err).Error("can't get workers worked tim from Hubstaff")
		return
	}
	var message = prefix + "\n"
	for _, separatedDate := range datesReports {
		if separatedDate.TimeWorked == 0 {
			continue
		}
		//separatedDate print
		message += fmt.Sprintf("\n\n\n*%s*", separatedDate.Date)
		for _, worker := range separatedDate.Users {
			//employee name print
			message += fmt.Sprintf("\n\n\n*%s (%s total)*\n", worker.Name, worker.TimeWorked)
			for _, project := range worker.Projects {
				message += fmt.Sprintf("\n%s - %s", project.TimeWorked, project.Name)
				for _, note := range project.Notes {
					message += fmt.Sprintf("\n - %s", note.Description)
				}
			}
		}
	}
	a.Slack.SendMessage(message, channel)
}

// ReportIsuuesWithClosedSubtasks create report about issues with closed subtasks
func (a *App) ReportIsuuesWithClosedSubtasks(channel string) {
	issues, err := a.Jira.IssuesWithClosedSubtasks()
	if err != nil {
		logrus.WithError(err).Error("can't take information about closed subtasks from jira")
		return
	}
	if len(issues) == 0 {
		a.Slack.SendMessage("There are no issues with all closed subtasks", channel)
		return
	}
	msgBody := a.Slack.ProjectManager + "\nIssues have all closed subtasks:\n"
	for _, issue := range issues {
		if issue.Fields.Status.Name != jira.StatusReadyForDemo {
			msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_\n",
				issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
		}
		if issue.Fields.Status.Name != jira.StatusCloseLastTask {
			err := a.Jira.IssueSetStatusCloseLastTask(issue.Key)
			if err != nil {
				logrus.WithError(err).Error("can't set PM review status for issue")
			}
		}
	}
	a.Slack.SendMessage(msgBody, channel)
}

// ReportEmployeesWithExceededEstimateTime create report about employees with ETA overhead
func (a *App) ReportEmployeesWithExceededEstimateTime(channel string) {
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
		a.Slack.SendMessage("There are no issues with remaining ETA.", channel)
		return
	}
	usersReports, err := a.Hubstaff.UsersWorkTimeByMember(now.BeginningOfWeek(), now.EndOfWeek())
	if err != nil {
		logrus.WithError(err).Error("can't get logged time from Hubstaff")
		return
	}
	// prepare the content
	messageHeader := fmt.Sprintf("\nExceeded estimate time report:\n\n*%v*\n", now.BeginningOfDay().Format("02.01.2006"))
	message := ""
	var maxWeekWorkingHours float32 = 30.0
	for _, userReport := range usersReports {
		if jiraRemainingEtaMap[userReport.Email] > 0 {
			workVolume := float32(jiraRemainingEtaMap[userReport.Email]+int(userReport.TimeWorked)) / 3600.0
			if workVolume > maxWeekWorkingHours {
				message += fmt.Sprintf("\n%s late for %.2f hours", userReport.Name, workVolume-maxWeekWorkingHours)
			}
		}
	}
	if message == "" {
		message = "No one developer has exceeded estimate time"
	}
	a.Slack.SendMessage(fmt.Sprintf("%s\n%s", messageHeader, message), channel)
}

// ReportEmployeesHaveExceededTasks create report about employees that have exceeded tasks
func (a *App) ReportEmployeesHaveExceededTasks(channel string) {
	issues, err := a.Jira.AssigneeOpenIssues()
	if err != nil {
		logrus.WithError(err).Error("can't take information about exceeded tasks of employees from jira")
		return
	}
	if len(issues) == 0 {
		a.Slack.SendMessage("There are no employees with exceeded subtasks", channel)
		return
	}

	msgBody := "Employees have exceeded tasks:\n"
	var developers = make(map[string][]jira.Issue)
	for _, issue := range issues {
		developer := ""
		// Convert to marshal map to find developer displayName of issue developer field
		developerMap, err := issue.Fields.Unknowns.MarshalMap(jira.FieldDeveloperMap)
		if err != nil {
			//can't make customfield_10026 map marshaling because field developer is empty
			developer = "No developer"
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
	for _, dev := range a.Slack.IgnoreList {
		delete(developers, dev)
	}
	var messageNoDeveloper string
	for developer, issues := range developers {
		var message string
		for _, issue := range issues {
			var worklogString string
			if issue.Fields.TimeTracking.TimeSpentSeconds > issue.Fields.TimeTracking.OriginalEstimateSeconds {
				worklogString = fmt.Sprintf(" time spent is %s instead %s", issue.Fields.TimeTracking.TimeSpent, issue.Fields.TimeTracking.OriginalEstimate)
			}
			message += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_%[4]s\n",
				issue.Key, issue.Fields.Summary, issue.Fields.Status.Name, worklogString)
		}
		switch {
		case developer == "No developer" && message != "":
			messageNoDeveloper += "\nAssigned issues without developer:\n" + message
		case message != "":
			msgBody += fmt.Sprintf("\n" + developer + "\n" + message)
		}
	}
	msgBody += messageNoDeveloper
	a.Slack.SendMessage(msgBody, channel)
}

// ReportIsuuesAfterSecondReview create report about issues after second review round
func (a *App) ReportIsuuesAfterSecondReview(channel string) {
	issues, err := a.Jira.IssuesAfterSecondReview()
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues after second review from jira")
		return
	}
	if len(issues) == 0 {
		a.Slack.SendMessage("There are no issues after second review round", channel)
		return
	}
	msgBody := "Issues after second review round:\n"
	for _, issue := range issues {
		msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_\n",
			issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
	}
	a.Slack.SendMessage(msgBody, channel)
}

// ReportSlackEndingFreeSpace create report about employees that have exceeded tasks
func (a *App) ReportSlackEndingFreeSpace(channel string) {
	size, err := a.Slack.FilesSize()
	if err != nil {
		logrus.WithError(err).Error("can't take information about files size from slack")
		return
	}
	if a.Slack.TotalVolume-size > a.Slack.RestVolume {
		return
	}
	msgBody := fmt.Sprintf("Free space on slack end.\n")
	a.Slack.SendMessage(msgBody, channel)
}

// ReportGitMigrations create report about new git migrations
func (a *App) ReportGitMigrations(channel string) {
	messages, err := a.MigrationMessages()
	if err != nil {
		logrus.WithError(err).Error("can't take information git migrations from bitbucket")
		return
	}
	for _, message := range messages {
		if message != "" {
			a.Slack.SendMessage(message, channel)
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

// ReportCurrentActivityWithCallback posts last activity to slack to defined callbackUrl
func (a *App) ReportCurrentActivityWithCallback(callbackURL string) {
	activitiesList, err := a.Hubstaff.CurrentActivity()
	if err != nil {
		logrus.WithError(err).Error("Can't get last activity report from Hubstaff.")
		return
	}
	message := a.stringFromCurrentActivitiesList(activitiesList)
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

// ReportCurrentActivity create report and send it to slack
func (a *App) ReportCurrentActivity(channel string) {
	activitiesList, err := a.Hubstaff.CurrentActivity()
	if err != nil {
		logrus.WithError(err).Error("Can't get last activity report from Hubstaff.")
		return
	}
	message := a.stringFromCurrentActivitiesList(activitiesList)
	a.Slack.SendMessage(message, channel)
}

// stringFromCurrentActivitiesList convert slice of last activities in string message report
func (a *App) stringFromCurrentActivitiesList(activitiesList []hubstaff.LastActivity) string {
	var (
		usersAtWork    string
		usersNotAtWork string
	)
	for _, activity := range activitiesList {
		if activity.ProjectName == "Not at work at the moment" {
			usersNotAtWork += fmt.Sprintf("\n\n*%s*\n%s", activity.User.Name, activity.ProjectName)
			continue
		}
		if activity.ProjectName != "" {
			usersAtWork += fmt.Sprintf("\n\n*%s*\n%s", activity.User.Name, activity.ProjectName)
			if activity.TaskJiraKey != "" {
				usersAtWork += fmt.Sprintf(" <https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>",
					activity.TaskJiraKey, activity.TaskSummary)
			}
		}
	}
	return usersAtWork + usersNotAtWork
}

// ReportSprintsIsuues create report about Completed issues, Completed but not verified, Issues left for the next, Issues in next sprint
func (a *App) ReportSprintsIsuues(project, channel string) error {
	issuesWithClosedStatus, err := a.Jira.IssuesClosedFromOpenSprint(project)
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues with closed status from jira")
		return err
	}
	issuesWithClosedSubtasks, err := a.Jira.IssuesClosedSubtasksFromOpenSprint(project)
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues with closed subtasks from jira")
		return err
	}
	issuesForNextSprint, err := a.Jira.IssuesForNextSprint(project)
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues stands for next sprint from jira")
		return err
	}
	issuesFromFutureSprint, err := a.Jira.IssuesFromFutureSprint(project)
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues from future sprint from jira")
		return err
	}
	var textIssuesReport string
	textIssuesReport += a.textMessageAboutIssuesStatus("Completed issues", issuesWithClosedStatus)
	textIssuesReport += a.textMessageAboutIssuesStatus("Completed, but not verified", issuesWithClosedSubtasks)
	textIssuesReport += a.textMessageAboutIssuesStatus("Issues left for the next sprint", issuesForNextSprint)
	textIssuesReport += a.textMessageAboutIssuesStatus("Issues from future sprint", issuesFromFutureSprint)
	a.Slack.SendMessage(textIssuesReport, channel)

	sprintInterface, ok := issuesWithClosedSubtasks[0].Fields.Unknowns[jira.FieldSprintInfo].([]interface{})
	if !ok {
		logrus.WithError(err).Error("can't parse interface from map")
		return fmt.Errorf("can't parse to interface: %v", issuesWithClosedSubtasks[0].Fields.Unknowns[jira.FieldSprintInfo])
	}
	sprintSequence, err := a.FindLastSprintSequence(sprintInterface)
	if err != nil {
		logrus.WithError(err).Error("can't find sprint of closed subtasks")
		return err
	}
	err = a.CreateIssuesCsvReport(issuesWithClosedSubtasks, fmt.Sprintf("Spring %v Closing", sprintSequence-1), channel, true)
	if err != nil {
		logrus.WithError(err).Error("can't create report of issues with closed subtasks from jira")
		return err
	}
	for _, issue := range issuesFromFutureSprint {
		issuesForNextSprint = append(issuesForNextSprint, issue)
	}
	err = a.CreateIssuesCsvReport(issuesForNextSprint, fmt.Sprintf("Spring %v Open", sprintSequence), channel, false)
	if err != nil {
		logrus.WithError(err).Error("can't create report of issues stands for next sprint from jira")
		return err
	}
	return nil
}

// textMessageAboutIssuesStatus create text message for report about issues
func (a *App) textMessageAboutIssuesStatus(messagePrefix string, issues []jira.Issue) string {
	var message string
	for _, issue := range issues {
		message += fmt.Sprintf("%s %s %s\n", issue.Fields.Type.Name, issue.Key, issue.Fields.Summary)
	}
	if message == "" {
		message += fmt.Sprintf("- %[1]s:\nThere are no %[1]s\n", messagePrefix)
		return message
	}
	message = fmt.Sprintf("- %s:\n", messagePrefix) + message
	return message
}

// CreateIssuesCsvReport create csv file with report about issues
func (a *App) CreateIssuesCsvReport(issues []jira.Issue, filename, channel string, withAdditionalInfo bool) error {
	if len(issues) == 0 {
		a.Slack.SendMessage("There are no issues for "+filename+" file", channel)
		return nil
	}
	file, err := os.Create(filename + ".csv")
	if err != nil {
		return err
	}

	writer := csv.NewWriter(file)
	if withAdditionalInfo {
		err = writer.Write([]string{"Type", "Key", "Summary", "Status", "Epic"})
		if err != nil {
			return err
		}
		for _, issue := range issues {
			epicName := "empty"
			if issue.Fields.Unknowns[jira.FieldEpicKey] != nil {
				epicName, err = a.Jira.EpicName(fmt.Sprint(issue.Fields.Unknowns[jira.FieldEpicKey]))
				if err != nil {
					logrus.WithError(err).Error("can't get issue summary from jira")
				}
			}
			err = writer.Write([]string{issue.Fields.Type.Name, issue.Key, issue.Fields.Summary, issue.Fields.Status.Name, epicName})
			if err != nil {
				return err
			}
		}
	}
	if !withAdditionalInfo {
		err = writer.Write([]string{"Type", "Key", "Summary"})
		if err != nil {
			return err
		}
		for _, issue := range issues {
			err = writer.Write([]string{issue.Fields.Type.Name, issue.Key, issue.Fields.Summary})
			if err != nil {
				return err
			}
		}
	}
	writer.Flush()
	file.Close()
	return a.SendFileToSlack(channel, filename+".csv")
}

// FindLastSprintSequence will find sequence of sprint from issue.Fields.Unknowns["customfield_10010"].([]interface{})
func (a *App) FindLastSprintSequence(sprints []interface{}) (int, error) {
	var lastSequence = 0
	rSeq, err := regexp.Compile(`sequence=(\d+)`)
	if err != nil {
		return 0, err
	}
	for i := range sprints {
		s, ok := sprints[i].(string)
		if !ok {
			return 0, fmt.Errorf("can't parse to string: %v", sprints[i])
		}
		// Find string submatch and get slice of match string and this sequence
		// For example, one of sprint:
		// "com.atlassian.greenhopper.service.sprint.Sprint@6f00eb7b[id=47,rapidViewId=12,state=ACTIVE,name=Sprint 46,
		// goal=,startDate=2019-02-20T04:19:23.907Z,endDate=2019-02-25T04:19:00.000Z,completeDate=<null>,sequence=47]"
		// we get string submatch of slice ["sequence=47" "47"] and then parse "47" as integer number to find the biggest one
		m := rSeq.FindStringSubmatch(s)
		if len(m) != 2 {
			return 0, fmt.Errorf("can't find submatch string to sequence: %v", sprints[i])
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, err
		}
		if n > lastSequence {
			lastSequence = n
		}
	}
	return lastSequence, nil
}

// SendFileToSlack sends file to slack
func (a *App) SendFileToSlack(channel, fileName string) error {
	fileDir, err := os.Getwd()
	if err != nil {
		return err
	}
	filePath := path.Join(fileDir, fileName)
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}
	writer.Close()
	err = a.Slack.UploadFile(channel, writer.FormDataContentType(), body)
	if err != nil {
		return err
	}
	file.Close()
	os.Remove(filePath)
	return nil
}

// ReportSprintStatus create report about sprint status
func (a *App) ReportSprintStatus(channel string) {
	issues, err := a.Jira.IssuesOfOpenSprints()
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues of open sprint from jira")
		return
	}
	msgBody := a.Slack.ProjectManager + "\n*Sprint status:*\n"
	if len(issues) == 0 {
		a.Slack.SendMessage(msgBody+"Open issues was not found. All issues of open sprint was closed.", channel)
		return
	}
	var developers = make(map[string][]jira.Issue)
	for _, issue := range issues {
		developer := "No developer"
		// Convert to marshal map to find developer displayName of issue field customfield_10026
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
	for _, dev := range a.Slack.IgnoreList {
		delete(developers, dev)
	}
	for developer, issues := range developers {
		var message string
		for _, issue := range issues {
			if issue.Fields.Status.Name != jira.StatusClosed && issue.Fields.Status.Name != jira.StatusInClarification {
				if issue.Fields.Parent != nil {
					message += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s> / ", issue.Fields.Parent.Key)
				}
				message += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_\n",
					issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
			}
		}
		if message != "" {
			msgBody += fmt.Sprintf(developer + " - has open tasks:\n" + message)
			continue
		}
		msgBody += fmt.Sprintf(" " + developer + " - all tasks closed.\n")
	}
	a.Slack.SendMessage(msgBody, channel)
}

// PersonActivityByDate create report about user activity and send messange about it
func (a *App) PersonActivityByDate(userName, date, channel string) error {
	email, err := a.Slack.UserEmailByName(strings.TrimPrefix(userName, "@"))
	if err != nil {
		return err
	}
	layout := "2006-01-02"
	t, err := time.Parse(layout, date)
	if err != nil {
		return err
	}
	userReport, userName, err := a.Hubstaff.UserWorkTimeByDate(t, t, email)
	if err != nil {
		return err
	}
	if userName == "" {
		return fmt.Errorf("Error! This user is not exist!")
	}
	var report string
	for _, worker := range userReport.Users {
		report += fmt.Sprintf("%s\n\n*%s (%s total)*\n", date, worker.Name, worker.TimeWorked)
		for _, project := range worker.Projects {
			report += fmt.Sprintf("\n%s - %s", project.TimeWorked, project.Name)
			for _, note := range project.Notes {
				report += fmt.Sprintf("\n - %s", note.Description)
			}
		}
	}
	if report == "" {
		report += fmt.Sprintf("%s\n\n*%s*\n\nHas not worked", date, userName)
	}
	a.Slack.SendMessage(report, channel)
	return nil
}
