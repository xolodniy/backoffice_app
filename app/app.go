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
	"sync"
	"time"

	"backoffice_app/common"
	"backoffice_app/config"
	"backoffice_app/model"
	"backoffice_app/services/bitbucket"
	"backoffice_app/services/hubstaff"
	"backoffice_app/services/jira"
	"backoffice_app/services/slack"

	"github.com/jinzhu/gorm"
	"github.com/jinzhu/now"
	_ "github.com/lib/pq"
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
	Hubstaff  hubstaff.Hubstaff
	Slack     slack.Slack
	Jira      jira.Jira
	Bitbucket bitbucket.Bitbucket
	Config    config.Main
	AfkTimer  AfkTimer
	model     model.Model
}

// AfkTimer struct for cache of user's AFK duration with mutex defend
type AfkTimer struct {
	*sync.Mutex
	UserDurationMap map[string]time.Duration
}

// New is main App constructor
func New(conf *config.Main) *App {
	db, err := gorm.Open("postgres", conf.Database.ConnURL())
	if err != nil {
		logrus.WithError(err).Fatal("can't open connection with a database")
	}
	if err := db.DB().Ping(); err != nil {
		logrus.WithError(err).Fatal("can't ping connection with a database")
	}

	model := model.New(db)
	if err := model.CheckMigrations(); err != nil {
		logrus.WithError(err).Fatal("invalid database condition")
	}
	return &App{
		Hubstaff:  hubstaff.New(&conf.Hubstaff),
		Slack:     slack.New(&conf.Slack),
		Jira:      jira.New(&conf.Jira),
		Bitbucket: bitbucket.New(&conf.Bitbucket),
		Config:    *conf,
		AfkTimer:  AfkTimer{Mutex: &sync.Mutex{}, UserDurationMap: make(map[string]time.Duration)},
		model:     model,
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
		message += separatedDate.String()
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
	msgBody := "\n*Issues have all closed subtasks:*\n\n"
	var designMessage string
	for _, issue := range issues {
		if issue.Fields.Status.Name != jira.StatusCloseLastTask {
			err := a.Jira.IssueSetStatusTransition(issue.Key, jira.StatusCloseLastTask)
			if err != nil {
				logrus.WithError(err).Errorf("can't set close last task transition for issue %s", issue.Key)
			}
		}
		switch {
		case issue.Fields.Status.Name == jira.StatusDesignReview:
			designMessage += issue.String()
		case issue.Fields.Status.Name != jira.StatusReadyForDemo:
			msgBody += issue.String()
		}
	}
	msgBody = msgBody + "cc " + a.Slack.Employees.ProjectManager + "\n\n" + designMessage + "cc " + a.Slack.Employees.ArtDirector
	a.Slack.SendMessage(msgBody, channel)
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
		developer := issue.DeveloperMap(jira.TagDeveloperName)
		if developer == "" {
			developer = jira.NoDeveloper
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
			if issue.Fields.TimeTracking.TimeSpentSeconds > issue.Fields.TimeTracking.OriginalEstimateSeconds {
				worklogString := fmt.Sprintf(" time spent is %s instead %s", issue.Fields.TimeTracking.TimeSpent, issue.Fields.TimeTracking.OriginalEstimate)
				message += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_%[4]s\n",
					issue.Key, issue.Fields.Summary, issue.Fields.Status.Name, worklogString)
			}
		}
		switch {
		case developer == jira.NoDeveloper && message != "":
			messageNoDeveloper += "\nAssigned issues without developer:\n" + message
		case message != "":
			msgBody += fmt.Sprintf("\n" + developer + "\n" + message)
		}
	}
	msgBody += messageNoDeveloper
	a.Slack.SendMessage(msgBody, channel)
}

// ReportIssuesAfterSecondReview create report about issues after second review round
func (a *App) ReportIssuesAfterSecondReview(channel string, issueTypes ...string) {
	issues, err := a.Jira.IssuesAfterSecondReview(issueTypes)
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues after second review from jira")
		return
	}
	if len(issues) == 0 {
		a.Slack.SendMessage("*Issues after second review round:*\n\nThere are no issues after second review round", channel)
		return
	}
	msgBody := "*Issues after second review round:*\n\n"
	for _, issue := range issues {
		msgBody += issue.String()
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

// FillCache fill commits caches for searching new migrations and new changes of ansible
func (a *App) FillCache() {
	migrationCommits, err := a.model.GetCommitsByType(common.CommitTypeMigration)
	if err != nil {
		logrus.WithError(err).Error("can't take commits cache from database")
		return
	}

	ansibleCommits, err := a.model.GetCommitsByType(common.CommitTypeAnsible)
	if err != nil {
		logrus.WithError(err).Error("can't take commits cache from database")
		return
	}
	// if commits cache is not empty return
	if len(migrationCommits) != 0 && len(ansibleCommits) != 0 {
		return
	}
	commits, err := a.Bitbucket.CommitsOfOpenedPRs()
	if err != nil {
		logrus.WithError(err).Error("can't take information about opened commits from bitbucket")
		return
	}

	if len(migrationCommits) == 0 {
		SQLCommits, err := a.SQLCommitsCache(commits)
		if err != nil {
			logrus.WithError(err).Error("can't take diff information from bitbucket")
			return
		}
		err = a.CreateCommitsCache(SQLCommits)
		if err != nil {
			logrus.WithError(err).Error("can't create commits cache in database")
			return
		}
	}

	if len(ansibleCommits) == 0 {
		AnsibleCommits, err := a.AnsibleCommitsCache(commits)
		if err != nil {
			logrus.WithError(err).Error("can't take diff information from bitbucket")
			return
		}

		err = a.CreateCommitsCache(AnsibleCommits)
		if err != nil {
			logrus.WithError(err).Error("can't create commits cache in database")
			return
		}
	}
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
	for _, commit := range newCommitsCache {
		dbCommit, err := a.model.GetCommitByHash(commit.Type, commit.Hash)
		if err != nil {
			logrus.WithError(err).Error("can't take commit from database")
			return nil, err
		}
		if len(dbCommit) == 0 {
			file, err := a.Bitbucket.SrcFile(commit.Repository, commit.Hash, commit.Path)
			if err != nil {
				logrus.WithError(err).Error("can't take information about file from bitbucket")
				return []string{}, err
			}
			files = append(files, commit.Message+"\n```"+file+"```\n")
		}
	}
	err = a.model.DeleteCommitsByType(common.CommitTypeMigration)
	if err != nil {
		logrus.WithError(err).Error("can't clear old commits cache from database")
	}
	err = a.CreateCommitsCache(newCommitsCache)
	if err != nil {
		logrus.WithError(err).Error("can't create commits cache in database")
	}
	return files, nil
}

// SQLCommitsCache returns commits cache with sql migration
func (a *App) SQLCommitsCache(commits []bitbucket.Commit) ([]model.Commit, error) {
	var newSQLCommits []model.Commit
	for _, commit := range commits {
		diffStats, err := a.Bitbucket.CommitsDiffStats(commit.Repository.Name, commit.Hash)
		if err != nil {
			return nil, err
		}
		for _, diffStat := range diffStats {
			if strings.Contains(diffStat.New.Path, ".sql") {
				newSQLCommits = append(newSQLCommits, model.Commit{
					Type:       common.CommitTypeMigration,
					Hash:       commit.Hash,
					Repository: commit.Repository.Name,
					Path:       diffStat.New.Path,
					Message:    commit.Message,
				})
			}
		}
	}
	return newSQLCommits, nil
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
	var usersAtWork string
	for _, activity := range activitiesList {
		if activity.ProjectName != "" {
			usersAtWork += fmt.Sprintf("\n\n*%s*\n%s", activity.User.Name, activity.ProjectName)
			if activity.TaskJiraKey != "" {
				usersAtWork += fmt.Sprintf(" <https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>",
					activity.TaskJiraKey, activity.TaskSummary)
			}
		}
	}
	if usersAtWork == "" {
		usersAtWork = "All users are not at work at the moment"
	}
	return usersAtWork
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
	textIssuesReport += a.textMessageAboutIssuesStatus("Closed issues (deployed to staging and verified)", issuesWithClosedStatus)
	textIssuesReport += a.textMessageAboutIssuesStatus("Issues in verification (done and deployed to staging but NOT yet verified)", issuesWithClosedSubtasks)
	textIssuesReport += a.textMessageAboutIssuesStatus("Issues which are still in development", issuesForNextSprint)
	textIssuesReport += a.textMessageAboutIssuesStatus("Issues from future sprint", issuesFromFutureSprint)
	a.Slack.SendMessage(textIssuesReport, channel)

	issuesBugStoryOfOpenSprint, err := a.Jira.IssuesStoryBugOfOpenSprints(project)
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues of open sprint from jira")
		return err
	}
	sprintInterface, ok := issuesBugStoryOfOpenSprint[0].Fields.Unknowns[jira.FieldSprintInfo].([]interface{})
	if !ok {
		logrus.WithError(err).Error("can't parse interface from map")
		return fmt.Errorf("can't parse to interface: %v", issuesWithClosedSubtasks[0].Fields.Unknowns[jira.FieldSprintInfo])
	}
	sprintSequence, err := a.FindLastSprintSequence(sprintInterface)
	if err != nil {
		logrus.WithError(err).Error("can't find sprint of closed subtasks")
		return err
	}
	for _, issue := range issuesFromFutureSprint {
		issuesForNextSprint = append(issuesForNextSprint, issue)
	}
	err = a.CreateIssuesCsvReport(issuesForNextSprint, fmt.Sprintf("Sprint %v Open", sprintSequence), channel, false)
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
	openIssues, err := a.Jira.OpenIssuesOfOpenSprints()
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues of open sprint from jira")
		return
	}
	sprintInterface, ok := openIssues[0].Fields.Unknowns[jira.FieldSprintInfo].([]interface{})
	if !ok {
		logrus.WithError(err).Error("can't parse interface from map")
		return
	}
	startDate, endDate, err := a.FindLastSprintDates(sprintInterface)
	if err != nil {
		logrus.WithError(err).Error("can't find sprint of open issue")
		return
	}
	closedIssues, err := a.Jira.IssuesClosedInInterim(startDate.AddDate(0, 0, -1), endDate.AddDate(0, 0, +1))
	if err != nil {
		logrus.WithError(err).Error("can't get closed issues of open sprint from jira")
		return
	}
	msgBody := "*Sprint status*\n"
	if len(openIssues) == 0 {
		a.Slack.SendMessage(msgBody+"Open issues was not found. All issues of open sprint was closed.", channel)
		return
	}
	var developers = make(map[string][]jira.Issue)
	for _, issue := range openIssues {
		developer := issue.DeveloperMap(jira.TagDeveloperName)
		if developer == "" {
			developer = "No developer"
		}
		developers[developer] = append(developers[developer], issue)
	}
	for _, issue := range closedIssues {
		developer := issue.DeveloperMap(jira.TagDeveloperName)
		if developer == "" {
			developer = jira.NoDeveloper
		}
		developers[developer] = append(developers[developer], issue)
	}
	for _, dev := range a.Slack.IgnoreList {
		delete(developers, dev)
	}
	var (
		messageAllTaskClosed string
		messageNoDeveloper   string
	)
	for developer, issues := range developers {
		var message string
		for _, issue := range issues {
			if issue.Fields.Status.Name != jira.StatusClosed && issue.Fields.Status.Name != jira.StatusInClarification {
				if developer == jira.NoDeveloper && issue.Fields.Assignee == nil {
					continue
				}
				if issue.Fields.Parent != nil {
					message += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s> / ", issue.Fields.Parent.Key)
				}
				message += issue.String()
			}
		}
		switch {
		case developer == jira.NoDeveloper && message != "":
			messageNoDeveloper += "\nAssigned issues without developer:\n" + message
		case message == "" && developer != jira.NoDeveloper:
			messageAllTaskClosed += fmt.Sprintf(developer + " - all tasks closed.\n")
		case message != "":
			msgBody += fmt.Sprintf("\n" + developer + " - has open tasks:\n" + message)
		}
	}
	msgBody += messageNoDeveloper + "\n" + messageAllTaskClosed
	a.Slack.SendMessage(msgBody+"\ncc "+a.Slack.Employees.ProjectManager, channel)
}

// ReportClarificationIssues create report about issues with clarification status
func (a *App) ReportClarificationIssues() {
	issues, err := a.Jira.ClarificationIssuesOfOpenSprints()
	if err != nil {
		logrus.WithError(err).Error("can't take information about issues with clarification status from jira")
		return
	}
	var assignees = make(map[string][]jira.Issue)
	for _, issue := range issues {
		assignees[issue.Fields.Assignee.Name] = append(assignees[issue.Fields.Assignee.Name], issue)
	}
	for _, issues := range assignees {
		var message string
		for _, issue := range issues {
			message += issue.String()
		}
		if message != "" {
			userId, err := a.Slack.UserIdByEmail(issues[0].Fields.Assignee.EmailAddress)
			if err != nil {
				logrus.WithError(err).Error("can't take user id by email from slack")
				continue
			}
			a.Slack.SendMessage("Issues with clarification status assigned to you:\n\n"+message, userId)
		}
	}

}

// PersonActivityByDate create report about user activity and send messange about it
func (a *App) PersonActivityByDate(userName, date, channel string) error {
	userInfo, err := a.Slack.UserInfoByName(strings.TrimPrefix(userName, "@"))
	if err != nil {
		return err
	}
	layout := "2006-01-02"
	t, err := time.Parse(layout, date)
	if err != nil {
		return err
	}
	userReport, err := a.Hubstaff.UserWorkTimeByDate(t, t, userInfo.Profile.Email)
	if err != nil {
		return err
	}
	report := userReport.String()
	if report == "" {
		report += fmt.Sprintf("%s\n\n*%s*\n\nHas not worked", date, "<@"+userInfo.Id+">")
	}
	a.Slack.SendMessage(report, channel)
	return nil
}

// Report24HoursReviewIssues create report about issues with long review status
func (a *App) Report24HoursReviewIssues() {
	issues, err := a.Jira.IssuesOnReview()
	if err != nil {
		logrus.WithError(err).Error("can't take information about not closed issues from jira")
		return
	}
	var assignees = make(map[string][]jira.Issue)
	for _, issue := range issues {
		timeWasCreated := issue.Changelog.Histories[len(issue.Changelog.Histories)-1].Created
		t, err := time.Parse("2006-01-02T15:04:05.999-0700", timeWasCreated)
		// if time empty or other format we continue to remove many log messages
		if err != nil {
			continue
		}
		if (time.Now().Unix() - t.Unix()) > 3600*24 {
			assignees[issue.Fields.Assignee.Name] = append(assignees[issue.Fields.Assignee.Name], issue)
		}
	}
	for _, issues := range assignees {
		var message string
		for _, issue := range issues {
			message += issue.String()
		}
		if message != "" {
			userId, err := a.Slack.UserIdByEmail(issues[0].Fields.Assignee.EmailAddress)
			if err != nil {
				logrus.WithError(err).Error("can't take user id by email from slack")
				continue
			}
			a.Slack.SendMessage("Issues on review more than 24 hours assigned to you:\n\n"+message, userId)
		}
	}
}

// ReportGitAnsibleChanges create report about new etc/ansible changes
func (a *App) ReportGitAnsibleChanges(channel string) {
	commits, err := a.Bitbucket.CommitsOfOpenedPRs()
	if err != nil {
		logrus.WithError(err).Error("can't take information about opened commits from bitbucket")
		return
	}

	newAnsibleCache, err := a.AnsibleCommitsCache(commits)
	if err != nil {
		logrus.WithError(err).Error("can't take diff information from bitbucket")
		return
	}
	var files []string
	for _, commit := range newAnsibleCache {
		dbCommit, err := a.model.GetCommitByHash(commit.Type, commit.Hash)
		if err != nil {
			logrus.WithError(err).Error("can't take commit from database")
			return
		}
		if len(dbCommit) == 0 {
			file, err := a.Bitbucket.SrcFile(commit.Repository, commit.Hash, commit.Path)
			if err != nil {
				logrus.WithError(err).Error("can't take information about file from bitbucket")
				return
			}
			files = append(files, commit.Message+"\n```"+file+"```\n")
		}
	}
	err = a.model.DeleteCommitsByType(common.CommitTypeAnsible)
	if err != nil {
		logrus.WithError(err).Error("can't clear old commits cache from database")
	}
	err = a.CreateCommitsCache(newAnsibleCache)
	if err != nil {
		logrus.WithError(err).Error("can't create commits cache in database")
	}
	for _, message := range files {
		a.Slack.SendMessage(message, channel)
	}
}

// AnsibleCommitsCache returns commits cache with etc/ansible changes
func (a *App) AnsibleCommitsCache(commits []bitbucket.Commit) ([]model.Commit, error) {
	var newAnsibleCommits []model.Commit
	for _, commit := range commits {
		diffStats, err := a.Bitbucket.CommitsDiffStats(commit.Repository.Name, commit.Hash)
		if err != nil {
			return nil, err
		}
		for _, diffStat := range diffStats {
			if strings.Contains(diffStat.New.Path, "etc/ansible") {
				newAnsibleCommits = append(newAnsibleCommits, model.Commit{
					Type:       common.CommitTypeAnsible,
					Hash:       commit.Hash,
					Repository: commit.Repository.Name,
					Path:       diffStat.New.Path,
					Message:    commit.Message,
				})
			}
		}
	}
	return newAnsibleCommits, nil
}

// MakeWorkersLessWorkedReportYesterday preparing a last day message of less worked users and send it to Slack
func (a *App) MakeWorkersLessWorkedReportYesterday(channel string) {
	a.ReportUsersLessWorked(
		now.BeginningOfDay().AddDate(0, 0, -1),
		now.EndOfDay().AddDate(0, 0, -1), channel)
}

// ReportUsersLessWorked send message to channel when users worked less then 6 hours
func (a *App) ReportUsersLessWorked(dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time, channel string) {
	usersReports, err := a.Hubstaff.UsersWorkTimeByMember(dateOfWorkdaysStart, dateOfWorkdaysEnd)
	if err != nil {
		logrus.WithError(err).Error("can't get workers worked time by member from Hubstaff")
		return
	}
	var users = make(map[string]string)
	for _, user := range usersReports {
		if int(user.TimeWorked) < 3600*6 {
			users[user.Name] = user.Email
		}
	}
	var (
		beTeamList string
		feTeamList string
	)
	for name, email := range users {
		userId, err := a.Slack.UserIdByEmail(email)
		if err != nil {
			logrus.WithError(err).Error("can't get user id by email from slack")
			userId = name
		}

		for _, developerName := range a.Slack.Employees.BeTeam {
			if developerName == name {
				beTeamList += fmt.Sprintf("<@%s> ", userId)
				break
			}
		}

		for _, developerName := range a.Slack.Employees.FeTeam {
			if developerName == name {
				feTeamList += fmt.Sprintf("<@%s> ", userId)
				break
			}
		}
	}
	if beTeamList != "" {
		a.Slack.SendMessage(fmt.Sprintf("%sworked less than 6 hours\nfyi %s %s",
			beTeamList, a.Slack.Employees.TeamLeaderBE, a.Slack.Employees.ProjectManager), channel)
	}
	if feTeamList != "" {
		a.Slack.SendMessage(fmt.Sprintf("%sworked less than 6 hours\nfyi %s %s",
			feTeamList, a.Slack.Employees.TeamLeaderFE, a.Slack.Employees.ProjectManager), channel)
	}
}

// StartAfkTimer starts timer while user is afk
func (a *App) StartAfkTimer(userDuration time.Duration, userId string) {
	err := a.model.CreateAfkTimer(model.AfkTimer{UserId: userId, Duration: userDuration.String()})
	if err != nil {
		logrus.WithError(err).Errorf("can't create afk timer in database")
	}
	a.AfkTimer.UserDurationMap[userId] = userDuration
	ticker := time.NewTicker(time.Second)
	go func() {
		for range ticker.C {
			a.AfkTimer.Lock()
			a.AfkTimer.UserDurationMap[userId] = a.AfkTimer.UserDurationMap[userId] - time.Second
			a.AfkTimer.Unlock()
			if a.AfkTimer.UserDurationMap[userId] <= 0 {
				ticker.Stop()
				err = a.model.DeleteAfkTimer(userId)
				if err != nil {
					logrus.WithError(err).Errorf("can't delete afk timer from database")
				}
			}
		}
	}()
}

// CheckUserAfkVacation check user on AFK and Vacation status
func (a *App) CheckUserAfkVacation(message, threadId, channel string) {
	for id, duration := range a.AfkTimer.UserDurationMap {
		if strings.Contains(message, id) && duration > 0 {
			userName, err := a.Slack.UserNameById(id)
			if err != nil {
				logrus.WithError(err).Errorf("can't take information about user name from slask with id: %v", id)
				userName = "This user"
			}
			a.Slack.SendToThread(fmt.Sprintf("%s will return in %.0f minutes", userName, duration.Minutes()), channel, threadId)
		}
	}

	vacations, err := a.model.GetActualVacations()
	if err != nil {
		if err == common.ErrNotFound {
			return
		}
		logrus.WithError(err).Errorf("can't take information about vacations from database")
	}
	for _, vacation := range vacations {
		if strings.Contains(message, vacation.UserId) {
			userName, err := a.Slack.UserNameById(vacation.UserId)
			if err != nil {
				logrus.WithError(err).Errorf("can't take information about user name from slask with id: %v", vacation.UserId)
				userName = "This user"
			}
			a.Slack.SendToThread(fmt.Sprintf("*%s* is on vacation, his message is: \n\n'%s'", userName, vacation.Message), channel, threadId)
		}
	}
}

// MessageIssueAfterSecondTLReview send message about issue after second tl review round
func (a *App) MessageIssueAfterSecondTLReview(issue jira.Issue) {
	if issue.Fields.Assignee == nil {
		return
	}
	reviewCount, err := a.Jira.RejectedIssueTLReviewCount(issue)
	if err != nil {
		logrus.WithError(err).Error("can't take information about review count after tl review from jira")
		return
	}
	if reviewCount < 2 {
		return
	}
	developerEmail := issue.DeveloperMap(jira.TagDeveloperEmail)
	var userId string
	switch developerEmail {
	case "":
		userId = jira.NoDeveloper
	default:
		userId, err = a.Slack.UserIdByEmail(developerEmail)
		if err != nil {
			logrus.WithError(err).Error("can't take user id by email from slack")
			userId = developerEmail
			break
		}
		userId = "<@" + userId + ">"
	}

	msgBody := fmt.Sprintf("The issue %s has been rejected after %v reviews\n\n", issue.Key, reviewCount)
	switch issue.Fields.Type.Name {
	case jira.TypeBESubTask, jira.TypeBETask:
		msgBody += fmt.Sprintf("Developer: %s\nfyi %s\nсс %s", userId, a.Slack.Employees.TeamLeaderBE, a.Slack.Employees.DirectorOfCompany)
	case jira.TypeFESubTask, jira.TypeFETask:
		msgBody += fmt.Sprintf("Developer: %s\nfyi %s\nсс %s", userId, a.Slack.Employees.TeamLeaderFE, a.Slack.Employees.DirectorOfCompany)
	default:
		return
	}
	a.Slack.SendMessage(msgBody, "#general")
}

// CreateCommitsCache creates commits in database
func (a *App) CreateCommitsCache(commits []model.Commit) error {
	for _, commit := range commits {
		err := a.model.CreateCommit(commit)
		if err != nil {
			return err
		}
	}
	return nil
}

// CheckAfkTimers checks saved afk timers and started it again
func (a *App) CheckAfkTimers() {
	afkTimers, err := a.model.GetAfkTimers()
	if err != nil {
		logrus.WithError(err).Errorf("can't take information about afk timers from database")
		return
	}
	for _, afkTimer := range afkTimers {
		duration, err := time.ParseDuration(afkTimer.Duration)
		if err != nil {
			logrus.WithError(err).Errorf("can't parse afk timer duration")
			continue
		}
		difference := time.Now().Sub(afkTimer.UpdatedAt)
		if difference < duration && difference > 0 {
			go a.StartAfkTimer(duration-difference, afkTimer.UserId)
			continue
		}
		err = a.model.DeleteAfkTimer(afkTimer.UserId)
		if err != nil {
			logrus.WithError(err).Errorf("can't delete afk timer")
		}
	}
}

// ReportOverworkedIssues create report about overworked issues
func (a *App) ReportOverworkedIssues(channel string) {
	issues, err := a.Jira.IssuesClosedInInterim(
		now.BeginningOfWeek().AddDate(0, 0, -8),
		now.EndOfWeek().AddDate(0, 0, -6))
	if err != nil {
		logrus.WithError(err).Error("can't get closed issues in interom from jira")
		return
	}
	var msgBody string
	var developers = make(map[string][]jira.Issue)
	for _, issue := range issues {
		developer := issue.DeveloperMap(jira.TagDeveloperName)
		if developer == "" {
			developer = jira.NoDeveloper
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
			overWorkedDuration := issue.Fields.TimeTracking.TimeSpentSeconds - issue.Fields.TimeTracking.OriginalEstimateSeconds
			if overWorkedDuration > issue.Fields.TimeTracking.OriginalEstimateSeconds/10 && issue.Fields.TimeTracking.RemainingEstimateSeconds == 0 {
				message += issue.String()
				message += fmt.Sprintf("- Time spent: %s\n", issue.Fields.TimeTracking.TimeSpent)
				message += fmt.Sprintf("- Time planned: %s\n", issue.Fields.TimeTracking.OriginalEstimate)
				message += fmt.Sprintf("- Overwork: %v\n", time.Duration(overWorkedDuration)*time.Second)
				message += fmt.Sprintf("- Overwork, %s: %v\n", "%%", overWorkedDuration/(issue.Fields.TimeTracking.OriginalEstimateSeconds/100))
			}
		}
		switch {
		case developer == jira.NoDeveloper && message != "":
			messageNoDeveloper += "\nAssigned issues without developer:\n" + message
		case message != "":
			msgBody += fmt.Sprintf("\n" + developer + "\n" + message)
		}
	}
	if msgBody == "" && messageNoDeveloper == "" {
		msgBody = "There are no issues with overworked time."
	}
	a.Slack.SendMessage("*Tasks time duration analyze*:\n"+msgBody+messageNoDeveloper, channel)
}

// FindLastSprintDates will find date of sprint from issue.Fields.Unknowns["customfield_10010"].([]interface{})
func (a *App) FindLastSprintDates(sprints []interface{}) (time.Time, time.Time, error) {
	var (
		startDate time.Time
		endDate   time.Time
	)
	sDate, err := regexp.Compile(`startDate=(\d{4}-\d{2}-\d{2})`)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	eDate, err := regexp.Compile(`endDate=(\d{4}-\d{2}-\d{2})`)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	for i := range sprints {
		s, ok := sprints[i].(string)
		if !ok {
			return time.Time{}, time.Time{}, fmt.Errorf("can't parse to string: %v", sprints[i])
		}
		// Find string submatch and get slice of match string and this startDate
		// For example, one of sprint:
		// "com.atlassian.greenhopper.service.sprint.Sprint@6f00eb7b[id=47,rapidViewId=12,state=ACTIVE,name=Sprint 46,
		// goal=,startDate=2019-02-20T04:19:23.907Z,endDate=2019-02-25T04:19:00.000Z,completeDate=<null>,sequence=47]"
		// we get string submatch of slice ["startDate=2019-02-20" "2019-02-20"] and then parse "2019-02-20" as time to find the biggest one
		sd := sDate.FindStringSubmatch(s)
		if len(sd) != 2 {
			return time.Time{}, time.Time{}, fmt.Errorf("can't find submatch string to startDate: %v", sprints[i])
		}
		ts, err := time.Parse("2006-01-02", sd[1])
		if err != nil {
			return time.Time{}, time.Time{}, err
		}

		ed := eDate.FindStringSubmatch(s)
		if len(ed) != 2 {
			return time.Time{}, time.Time{}, fmt.Errorf("can't find submatch string to endDate: %v", sprints[i])
		}
		te, err := time.Parse("2006-01-02", ed[1])
		if err != nil {
			return time.Time{}, time.Time{}, err
		}

		if ts.After(startDate) {
			startDate = ts
			endDate = te
		}
	}
	return startDate, endDate, nil
}

// SetVacationPeriod create vacation period for user
func (a *App) SetVacationPeriod(dateStart, dateEnd, message, userId string) error {
	dStart, err := time.Parse("02.01.2006", dateStart)
	if err != nil {
		return err
	}
	dEnd, err := time.Parse("02.01.2006", dateEnd)
	if err != nil {
		return err
	}
	if dStart.After(dEnd) {
		return fmt.Errorf("Date of start vacation bigger then data of end")
	}

	err = a.model.SaveVacation(model.Vacation{
		UserId:    userId,
		DateStart: dStart,
		DateEnd:   dEnd,
		Message:   message,
	})
	if err != nil {
		return err
	}
	return nil
}

// CancelVacation delete vacation
func (a *App) CancelVacation(userId string) error {
	_, err := a.CheckVacationSatus(userId)
	if err != nil {
		return err
	}
	err = a.model.DeleteVacation(userId)
	if err != nil {
		return err
	}
	return nil
}

// CheckVacationSatus get vacation if exist
func (a *App) CheckVacationSatus(userId string) (model.Vacation, error) {
	vacation, err := a.model.GetVacation(userId)
	if err != nil {
		return model.Vacation{}, err
	}
	return vacation, nil
}

// CreateIssueBranches create branch of issue and its parent
func (a *App) CreateIssueBranches(issue jira.Issue) {
	if issue.Fields.Status.Name != jira.StatusStarted {
		return
	}
	if issue.Fields.Parent == nil {
		err := a.Bitbucket.CreateBranch(issue.Key, issue.Key, "master")
		if err != nil {
			logrus.WithError(err).WithField("issueKey", fmt.Sprintf("%+v", issue.Key)).
				Error("can't create branch")
		}
		return
	}
	err := a.Bitbucket.CreateBranch(issue.Key, issue.Fields.Parent.Key, "master")
	if err != nil {
		logrus.WithError(err).WithField("issueKey", fmt.Sprintf("%+v", issue.Key)).
			Error("can't create branch")
		return
	}
	err = a.Bitbucket.CreateBranch(issue.Key, issue.Fields.Parent.Key+">"+issue.Key, issue.Fields.Parent.Key)
	if err != nil {
		logrus.WithError(err).WithField("issueKey", fmt.Sprintf("%+v", issue.Key)).
			Error("can't create branch")
		return
	}
}

// CreateBranchPullRequest create pull request for first branch commit
func (a *App) CreateBranchPullRequest(repoPushPayload bitbucket.RepoPushPayload) {
	// if commit was deleted or branch was deleted, new name will be empty, and we check it to do nothing
	if repoPushPayload.Push.Changes[0].New.Name == "" {
		return
	}
	if !strings.Contains(repoPushPayload.Push.Changes[0].New.Name, ">") {
		err := a.Bitbucket.CreatePullRequestIfNotExist(repoPushPayload.Repository.Slug, repoPushPayload.Push.Changes[0].New.Name, "master")
		if err != nil {
			logrus.WithError(err).WithField("branch", fmt.Sprintf("%+v", repoPushPayload.Push.Changes[0].New.Name)).
				Error("can't create pull request of branch")
		}
		return
	}

	issuesKey := strings.Split(repoPushPayload.Push.Changes[0].New.Name, ">")
	if len(issuesKey) != 2 {
		logrus.WithField("branchName", fmt.Sprintf("%+v", repoPushPayload.Push.Changes[0].New.Name)).
			Error("can't take issue key from branch name, format must be KEY-1/KEY-2")
		return
	}
	err := a.Bitbucket.CreatePullRequestIfNotExist(repoPushPayload.Repository.Name, repoPushPayload.Push.Changes[0].New.Name, issuesKey[0])
	if err != nil {
		logrus.WithError(err).WithField("branch", fmt.Sprintf("%+v", repoPushPayload.Push.Changes[0].New.Name)).
			Error("can't create pull request of branch")
		return
	}
}

// ReportEpicsWithClosedIssues create report about epics with closed issues
func (a *App) ReportEpicsWithClosedIssues(channel string) {
	epics, err := a.Jira.EpicsWithClosedIssues()
	if err != nil {
		logrus.WithError(err).Error("can't take information about epics with closed issues from jira")
		return
	}
	if len(epics) == 0 {
		a.Slack.SendMessage("There are no epics with all closed issues", channel)
		return
	}
	msgBody := "\n*Epics have all closed issues:*\n\n"
	for _, epic := range epics {
		if epic.Fields.Status.Name != jira.StatusInArtDirectorReview {
			err := a.Jira.IssueSetStatusTransition(epic.Key, jira.StatusInArtDirectorReview)
			if err != nil {
				logrus.WithError(err).Errorf("can't set close last task transition for issue %s", epic.Key)
			}
		}
		msgBody += epic.String()
	}
	msgBody += "cc " + a.Slack.Employees.ArtDirector
	a.Slack.SendMessage(msgBody, channel)
}
