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
	"sort"
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
	"backoffice_app/types"

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

// Tags for user map of account info
var (
	TagUserJiraAccountID = "jiraaccountid"
	TagUserEmail         = "email"
	TagUserSlackID       = "slackid"
	TagUserSlackName     = "slackname"
	TagUserSlackRealName = "slackrealname"
	EmptyTagValue        = "empty"
)

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
	var (
		generalMessage string
		designMessage  string
	)
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
		case issue.Fields.Status.Name == jira.StatusQAReview:
			continue
		case issue.Fields.Status.Name != jira.StatusReadyForDemo:
			generalMessage += issue.String()
		}
	}
	var msgBody string
	if generalMessage != "" {
		msgBody += generalMessage + "cc " + a.Slack.Employees.ProjectManager + "\n\n"
	}
	if designMessage != "" {
		msgBody += designMessage + "cc " + a.Slack.Employees.ArtDirector
	}
	if msgBody != "" {
		a.Slack.SendMessage("*Issues have all closed subtasks:*\n\n"+msgBody, channel)
	}
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

	var developerEmails = make(map[string][]jira.Issue)
	for _, issue := range issues {
		if common.ValueIn(issue.DeveloperMap(jira.TagDeveloperName), a.Slack.IgnoreList...) {
			continue
		}
		var developerEmail string
		developerID := issue.DeveloperMap(jira.TagDeveloperID)
		userInfo := a.GetUserInfoByTagValue(TagUserJiraAccountID, developerID)
		if userInfo[TagUserEmail] != "" {
			developerEmail = userInfo[TagUserEmail]
		} else {
			developerEmail = jira.NoDeveloper
		}
		developerEmails[developerEmail] = append(developerEmails[developerEmail], issue)
	}
	var (
		msgBody            string
		messageNoDeveloper string
	)
	for developerEmail, issues := range developerEmails {
		var message string
		for _, issue := range issues {
			if issue.Fields.TimeTracking.TimeSpentSeconds > issue.Fields.TimeTracking.OriginalEstimateSeconds && issue.Fields.TimeTracking.RemainingEstimateSeconds == 0 {
				worklogString := fmt.Sprintf(" time spent is %s instead %s", issue.Fields.TimeTracking.TimeSpent, issue.Fields.TimeTracking.OriginalEstimate)
				message += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_%[4]s\n",
					issue.Key, issue.Fields.Summary, issue.Fields.Status.Name, worklogString)
			}
		}
		switch {
		case developerEmail == jira.NoDeveloper && message != "":
			messageNoDeveloper += "\nAssigned issues without developer:\n" + message
		case message != "":
			userInfo := a.GetUserInfoByTagValue(TagUserEmail, developerEmail)
			if userInfo[TagUserSlackID] == "" {
				msgBody += fmt.Sprintf("\n" + developerEmail + "\n" + message)
				continue
			}
			msgBody += fmt.Sprintf("\n<@%s> "+"\n"+message, userInfo[TagUserSlackID])
		}
	}
	if msgBody == "" && messageNoDeveloper == "" {
		return
	}
	msgBody += messageNoDeveloper
	a.Slack.SendMessage("Employees have exceeded tasks:\n"+msgBody+"\n\ncc "+a.Slack.Employees.ProjectManager, channel)
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
		_, err := a.model.GetCommitByHash(commit.Hash)
		if err == common.ErrNotFound {
			file, err := a.Bitbucket.SrcFile(commit.Repository, commit.Hash, commit.Path)
			if err != nil {
				logrus.WithError(err).Error("can't take information about file from bitbucket")
				return []string{}, err
			}
			files = append(files, commit.Message+"\n```"+file+"```\n")
			continue
		}
		if err != nil {
			logrus.WithError(err).Error("can't take commit from database")
			return nil, err
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
	message := a.stringFromCurrentActivitiesWithNotes(activitiesList)
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
	message := a.stringFromCurrentActivitiesWithNotes(activitiesList)
	a.Slack.SendMessage(message, channel)
}

// stringFromCurrentActivitiesWithNotes convert slice of last activities in string message report
func (a *App) stringFromCurrentActivitiesWithNotes(activitiesList []hubstaff.LastActivity) string {
	var usersAtWork string
	for _, activity := range activitiesList {
		usersAtWork += fmt.Sprintf("\n\n*%s*\n%s", activity.User.Name, activity.ProjectName)
		if activity.TaskJiraKey != "" {
			usersAtWork += fmt.Sprintf(" <https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>",
				activity.TaskJiraKey, activity.TaskSummary)
		}
		note, err := a.Hubstaff.LastUserNote(strconv.Itoa(activity.User.ID), strconv.Itoa(activity.LastProjectID))
		if err != nil {
			logrus.WithError(err).Error("Can't get user last note for report from Hubstaff.")
			continue
		}
		if note.Description == "" {
			continue
		}
		loc := time.FixedZone("UTC3", 3*60*60)
		usersAtWork += fmt.Sprintf("\n ✎ %s (%s)", note.Description, note.RecordedAt.In(loc).Format(time.RFC822Z))
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
			developer = jira.NoDeveloper
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
		messageSummaryData   string
	)
	for developer, issues := range developers {
		var (
			message         string
			developerIssues []string
		)
		for _, issue := range issues {
			if issue.Fields.Status.Name != jira.StatusClosed && issue.Fields.Status.Name != jira.StatusInClarification {
				if developer == jira.NoDeveloper && issue.Fields.Assignee == nil {
					continue
				}
				if issue.Fields.Parent != nil {
					message += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s> / ", issue.Fields.Parent.Key)
				}
				message += issue.String()
				developerIssues = append(developerIssues, issue.Key)
			}
		}
		switch {
		case developer == jira.NoDeveloper && message != "":
			messageNoDeveloper += "\nAssigned issues without developer:\n" + message
			messageSummaryData += developer + " " + strings.Join(developerIssues, ",") + "\n"
		case message == "" && developer != jira.NoDeveloper:
			messageAllTaskClosed += fmt.Sprintf(developer + " - all tasks closed.\n")
		case message != "":
			msgBody += fmt.Sprintf("\n" + developer + " - has open tasks:\n" + message)
			messageSummaryData += developer + " " + strings.Join(developerIssues, ",") + "\n"
		}
	}
	msgBody += messageNoDeveloper + "\n" + messageAllTaskClosed
	a.Slack.SendMessage(msgBody+"\ncc "+a.Slack.Employees.ProjectManager, channel)
	a.Slack.SendMessage("*Summary sprint table:*\n```"+messageSummaryData+"```", channel)
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
		assignees[issue.Fields.Assignee.AccountID] = append(assignees[issue.Fields.Assignee.AccountID], issue)
	}
	for accountID, issues := range assignees {
		var message string
		for _, issue := range issues {
			message += issue.String()
		}
		if message != "" {
			userInfo := a.GetUserInfoByTagValue(TagUserJiraAccountID, accountID)
			if userInfo[TagUserSlackID] == EmptyTagValue {
				continue
			}
			if userInfo[TagUserSlackID] == "" {
				logrus.WithError(err).WithField("accountID", accountID).Error("can't take user id by accountID from vocabulary")
				continue
			}
			a.Slack.SendMessage("Issues with clarification status assigned to you:\n\n"+message, userInfo[TagUserSlackID])
		}
	}

}

// PersonActivityByDate create report about user activity and send messange about it
func (a *App) PersonActivityByDate(userName, date, channel string) error {
	userInfo := a.GetUserInfoByTagValue(TagUserSlackName, strings.TrimPrefix(userName, "@"))
	if userInfo[TagUserEmail] == "" {
		return fmt.Errorf("Данные пользователя не были найдены в словаре")
	}
	layout := "2006-01-02"
	t, err := time.Parse(layout, date)
	if err != nil {
		return err
	}
	userReport, err := a.Hubstaff.UserWorkTimeByDate(t, t, userInfo[TagUserEmail])
	if err != nil {
		return err
	}
	report := userReport.String()
	if report == "" {
		report += fmt.Sprintf("%s\n\n*%s*\n\nHas not worked", date, "<@"+userInfo[TagUserSlackID]+">")
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
			assignees[issue.Fields.Assignee.AccountID] = append(assignees[issue.Fields.Assignee.AccountID], issue)
		}
	}
	for accountID, issues := range assignees {
		var message string
		for _, issue := range issues {
			message += issue.String()
		}
		if message != "" {
			userInfo := a.GetUserInfoByTagValue(TagUserJiraAccountID, accountID)
			if userInfo[TagUserSlackID] == EmptyTagValue {
				continue
			}
			if userInfo[TagUserSlackID] == "" {
				logrus.WithError(err).WithField("accountID", accountID).Error("can't take user id by accountID from vocabulary")
				continue
			}
			a.Slack.SendMessage("Issues on review more than 24 hours assigned to you:\n\n"+message, userInfo[TagUserSlackID])
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
		_, err := a.model.GetCommitByHash(commit.Hash)
		if err == common.ErrNotFound {
			file, err := a.Bitbucket.SrcFile(commit.Repository, commit.Hash, commit.Path)
			if err != nil {
				logrus.WithError(err).Error("can't take information about file from bitbucket")
				return
			}
			files = append(files, commit.Message+"\n```"+file+"```\n")
			continue
		}
		if err != nil {
			logrus.WithError(err).Error("can't take commit from database")
			return
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
		userInfo := a.GetUserInfoByTagValue(TagUserEmail, email)
		if userInfo[TagUserSlackID] == EmptyTagValue {
			userInfo[TagUserSlackID] = name
		}
		if userInfo[TagUserSlackID] == "" {
			logrus.WithError(err).WithField("email", email).Error("can't take user id by email from vocabulary")
			userInfo[TagUserSlackID] = name
		}

		for _, developerName := range a.Slack.Employees.BeTeam {
			if developerName == name {
				beTeamList += fmt.Sprintf("<@%s> ", userInfo[TagUserSlackID])
				break
			}
		}

		for _, developerName := range a.Slack.Employees.FeTeam {
			if developerName == name {
				feTeamList += fmt.Sprintf("<@%s> ", userInfo[TagUserSlackID])
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
			userInfo := a.GetUserInfoByTagValue(TagUserSlackID, id)
			if userInfo[TagUserSlackRealName] == "" || userInfo[TagUserSlackRealName] == EmptyTagValue {
				logrus.Errorf("can't take information about user name from vocabulary with id: %v", id)
				userInfo[TagUserSlackRealName] = "This user"
			}
			a.Slack.SendToThread(fmt.Sprintf("*%s* will return in %s", userInfo[TagUserSlackRealName], common.FmtDuration(duration)), channel, threadId)
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
			userInfo := a.GetUserInfoByTagValue(TagUserSlackID, vacation.UserId)
			if userInfo[TagUserSlackRealName] == "" || userInfo[TagUserSlackRealName] == EmptyTagValue {
				logrus.Errorf("can't take information about user name from vocabulary with id: %v", vacation.UserId)
				userInfo[TagUserSlackRealName] = "This user"
			}
			a.Slack.SendToThread(fmt.Sprintf("*%s* is on vacation, his message is: \n\n'%s'", userInfo[TagUserSlackRealName], vacation.Message), channel, threadId)
		}
	}
}

// CheckAmplifyMessage check message from amplify and resend
func (a *App) CheckAmplifyMessage(channelID string, attachments []types.PostChannelMessageAttachment) {
	if channelID != a.Config.Amplify.NotifyChannelID {
		return
	}
	for _, attachment := range attachments {
		switch {
		case strings.Contains(attachment.Fallback, "Host: Staging"):
			a.Slack.SendMessageWithAttachments("", a.Config.Amplify.ChannelStag, attachments)
			return
		case strings.Contains(attachment.Fallback, "Host: Production"):
			var usersMention string
			for _, mention := range a.Config.Amplify.Mention {
				usersMention += "<@" + mention + "> "
			}
			a.Slack.SendMessageWithAttachments(usersMention, a.Config.Amplify.ChannelProd, attachments)
			return
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
	developerID := issue.DeveloperMap(jira.TagDeveloperID)
	userInfo := a.GetUserInfoByTagValue(TagUserJiraAccountID, developerID)
	var userId string
	switch {
	case userInfo[TagUserSlackID] == EmptyTagValue:
		userId = userInfo[TagUserEmail]
	case userInfo[TagUserSlackID] == "":
		userId = jira.NoDeveloper
	default:
		userId = "<@" + userInfo[TagUserSlackID] + ">"
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
	// sort by overwork %
	sort.SliceStable(issues, func(i, j int) bool {
		iEstimate := issues[i].Fields.TimeTracking.OriginalEstimateSeconds
		jEstimate := issues[j].Fields.TimeTracking.OriginalEstimateSeconds
		if iEstimate/100 == 0 || jEstimate/100 == 0 {
			return false
		}
		iTimeSpent := issues[i].Fields.TimeTracking.TimeSpentSeconds
		jTimeSpent := issues[j].Fields.TimeTracking.TimeSpentSeconds
		return (iTimeSpent-iEstimate)/(iEstimate/100) <
			(jTimeSpent - jEstimate/(jEstimate/100))
	})
	var msgBody string
	for _, issue := range issues {
		developer := issue.DeveloperMap(jira.TagDeveloperName)
		if developer == "" {
			developer = jira.NoDeveloper
		}
		if common.ValueIn(developer, a.Slack.IgnoreList...) {
			continue
		}
		overWorkedDuration := issue.Fields.TimeTracking.TimeSpentSeconds - issue.Fields.TimeTracking.OriginalEstimateSeconds
		if overWorkedDuration < issue.Fields.TimeTracking.OriginalEstimateSeconds/10 ||
			issue.Fields.TimeTracking.RemainingEstimateSeconds != 0 ||
			issue.Fields.TimeTracking.OriginalEstimateSeconds == 0 || overWorkedDuration < 60*60 ||
			issue.Fields.TimeTracking.OriginalEstimateSeconds/100 == 0 {
			continue
		}
		msgBody += "\n" + developer + "\n" + issue.String()
		msgBody += fmt.Sprintf("- Time spent: %s\n", issue.Fields.TimeTracking.TimeSpent)
		msgBody += fmt.Sprintf("- Time planned: %s\n", issue.Fields.TimeTracking.OriginalEstimate)
		msgBody += fmt.Sprintf("- Overwork: %v\n", time.Duration(overWorkedDuration)*time.Second)
		msgBody += fmt.Sprintf("- Overwork, %%: %v\n", overWorkedDuration/(issue.Fields.TimeTracking.OriginalEstimateSeconds/100))
	}
	if msgBody == "" {
		msgBody = "There are no issues with overworked time."
	}
	a.Slack.SendMessage("*Tasks time duration analyze*:\n"+msgBody, channel)
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
		err := a.Bitbucket.CreatePullRequestIfNotExist(repoPushPayload.Repository.Name, repoPushPayload.Push.Changes[0].New.Name, "master")
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"repoSlug":         repoPushPayload.Repository.Name,
				"branchName":       repoPushPayload.Push.Changes[0].New.Name,
				"parentBranchName": "master",
			}).Error("can't create pull request of branch")
		}
		return
	}

	issuesKey := strings.Split(repoPushPayload.Push.Changes[0].New.Name, ">")
	if len(issuesKey) != 2 {
		logrus.WithField("branchName", fmt.Sprintf("%+v", repoPushPayload.Push.Changes[0].New.Name)).
			Error("can't take issue key from branch name, format must be KEY-1>KEY-2")
		return
	}
	err := a.Bitbucket.CreatePullRequestIfNotExist(repoPushPayload.Repository.Name, repoPushPayload.Push.Changes[0].New.Name, issuesKey[0])
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"repoSlug":         repoPushPayload.Repository.Name,
			"branchName":       repoPushPayload.Push.Changes[0].New.Name,
			"parentBranchName": issuesKey[0],
		}).Error("can't create pull request of branch")
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
	var msgBody string
	for _, epic := range epics {
		if epic.Fields.Status.Name != jira.StatusInArtDirectorReview {
			err := a.Jira.IssueSetStatusTransition(epic.Key, jira.StatusInArtDirectorReview)
			if err != nil {
				logrus.WithError(err).Errorf("can't set close last task transition for issue %s", epic.Key)
			}
		}
		msgBody += epic.String()
	}

	if msgBody != "" {
		msgBody = "\n*Epics have all closed issues:*\n\n" + msgBody + "cc " + a.Slack.Employees.ArtDirector
		a.Slack.SendMessage(msgBody, channel)
	}
}

// MoveJiraStatuses move jira issues statuses
func (a *App) MoveJiraStatuses(issue jira.Issue) {
	if issue.Fields.Type.Subtask {
		parentType, err := a.Jira.IssueType(issue.Fields.Parent.ID)
		if err != nil {
			return
		}
		switch issue.Fields.Status.Name {
		case jira.StatusOpen:
			if parentType == jira.TypeStory {
				err := a.Jira.IssueSetStatusTransition(issue.Fields.Parent.ID, jira.TransitionCreatingDevSubtasks)
				if err != nil {
					return
				}
			}
		case jira.StatusStarted:
			if parentType == jira.TypeStory {
				err := a.Jira.IssueSetStatusTransition(issue.Fields.Parent.ID, jira.TransitionCompleteSubtasksCreation)
				if err != nil {
					return
				}
			}
			if parentType == jira.TypeBug {
				err := a.Jira.IssueSetStatusTransition(issue.Fields.Parent.ID, jira.TransitionStart)
				if err != nil {
					return
				}
			}
		case jira.StatusClosed:
			if parentType == jira.TypeBug && a.Jira.IssueSubtasksClosed(issue.Fields.Parent.ID) {
				err := a.Jira.IssueSetStatusTransition(issue.Fields.Parent.ID, jira.TransitionDone)
				if err != nil {
					return
				}
			}
		}
	}

	if issue.Fields.Type.Name == jira.TypeStory && issue.Fields.Unknowns[jira.FieldEpicKey] != nil {
		if issue.Fields.Status.Name == jira.StatusStarted {
			err := a.Jira.IssueSetStatusTransition(fmt.Sprint(issue.Fields.Unknowns[jira.FieldEpicKey]), jira.TransitionCloaseLastTask)
			if err != nil {
				return
			}
		}
		if issue.Fields.Status.Name == jira.StatusClosed && a.Jira.EpicIssuesClosed(fmt.Sprint(issue.Fields.Unknowns[jira.FieldEpicKey])) {
			err := a.Jira.IssueSetStatusTransition(fmt.Sprint(issue.Fields.Unknowns[jira.FieldEpicKey]), jira.TransitionDone)
			if err != nil {
				return
			}
		}
	}
}

// CheckPullRequestsConflicts checks pull requests on containing conflict
func (a *App) CheckPullRequestsConflicts(pullRequestPayload bitbucket.PullRequestMergedPayload) {
	pullRequests, err := a.Bitbucket.PullRequestsList(pullRequestPayload.Repository.Name)
	if err != nil {
		logrus.WithError(err).Errorf("Can't get pull requests list")
		return
	}
	var authorPullRequests = make(map[string][]string)
	for _, pullRequest := range pullRequests {
		diff, err := a.Bitbucket.PullRequestDiff(pullRequestPayload.Repository.Name, pullRequest.ID)
		if err != nil {
			logrus.WithError(err).Errorf("Can't get pull request diff")
			return
		}
		if strings.Contains(diff, "<<<<<<< destination") {
			authorPullRequests[pullRequest.Author.DisplayName] = append(authorPullRequests[pullRequest.Author.DisplayName], "<"+pullRequest.Links.HTML.Href+"|"+pullRequest.Title+">")
		}
	}
	for author, pullRequestsTitles := range authorPullRequests {
		msg := "*Pull requests with conflicts:*\n\n"
		for _, title := range pullRequestsTitles {
			msg += title + "\n"
		}
		a.Slack.SendMessage(msg, author)
	}
}

// GetUserInfoByTagValue retrieve user info by value of tag in map
func (a *App) GetUserInfoByTagValue(tag, value string) config.User {
	for _, a := range a.Config.Users {
		if a[tag] != "" && a[tag] == value {
			return a
		}
	}
	return make(config.User, 0)
}

// ChangeJiraSubtasksInfo change fix versions and priority of subtasks
func (a *App) ChangeJiraSubtasksInfo(issue jira.Issue, changelog jira.Changelog) {
	if len(issue.Fields.Subtasks) == 0 {
		return
	}
	for _, changelogItem := range changelog.Items {
		switch changelogItem.Field {
		case jira.ChangelogFieldFixVersion:
			for _, subtask := range issue.Fields.Subtasks {
				if err := a.Jira.UpdateIssueFixVersion(subtask.Key, changelogItem.FromString, changelogItem.ToString); err != nil {
					return
				}
			}
		case jira.ChangelogFieldPrioriy:
			for _, subtask := range issue.Fields.Subtasks {
				if err := a.Jira.SetIssuePriority(subtask.Key, changelogItem.ToString); err != nil {
					return
				}
			}
		case jira.ChangelogFieldDueDate:
			for _, subtask := range issue.Fields.Subtasks {
				if err := a.Jira.SetIssueDueDate(subtask.Key, changelogItem.ToString); err != nil {
					return
				}
			}
		}
	}
}

// ReportLowPriorityIssuesStarted checks if developer start issue with low priority and send report about it
func (a *App) ReportLowPriorityIssuesStarted(channel string) {
	// get all opened and started issues with one last worklog activity, sorted by priority from highest
	issues, err := a.Jira.OpenedIssuesWithLastWorklogActivity()
	if err != nil {
		logrus.WithError(err).Error("Can't get issue from Jira with last worklog activity")
		return
	}
	// sort by assignee
	assigneeIssues := make(map[string][]jira.Issue)
	for _, issue := range issues {
		if issue.Fields.Assignee == nil {
			continue
		}
		assigneeIssues[issue.Fields.Assignee.AccountID] = append(assigneeIssues[issue.Fields.Assignee.AccountID], issue)
	}
	hourAgoUTC := time.Now().UTC().Add(-1 * time.Hour)
	for developer, issues := range assigneeIssues {
		user := a.GetUserInfoByTagValue(TagUserJiraAccountID, developer)
		// check developers in ignore list
		if common.ValueIn(user[TagUserSlackRealName], a.Config.IgnoreList...) {
			continue
		}
		var activeIssue jira.Issue
		// set first issue as priority
		priorityIssue := issues[0]
		// find priority and active tasks to check, if active task not priority, send message
		for _, issue := range issues {
			if issue.Fields.Priority.ID < priorityIssue.Fields.Priority.ID {
				priorityIssue = issue
			}
			if len(issue.Fields.Worklog.Worklogs) == 0 {
				continue
			}
			// check if issue has activity, but not started and start it
			if issue.Fields.Status.Name == jira.StatusOpen {
				a.Jira.IssueSetStatusTransition(issue.Key, jira.TransitionStart)
			}

			if activeIssue.Fields == nil || len(activeIssue.Fields.Worklog.Worklogs) == 0 {
				activeIssue = issue
				continue
			}
			issueTimeStarted := *issue.Fields.Worklog.Worklogs[0].Started
			activeIssueTimeStarted := *activeIssue.Fields.Worklog.Worklogs[0].Started
			if time.Time(issueTimeStarted).After(time.Time(activeIssueTimeStarted)) {
				activeIssue = issue
			}
		}
		if activeIssue.Fields == nil || activeIssue.Fields.Worklog == nil || len(activeIssue.Fields.Worklog.Worklogs) == 0 {
			continue
		}
		if activeIssue.Fields.Priority.ID == priorityIssue.Fields.Priority.ID {
			activeReleaseDate := a.getNearestFixVersionDate(activeIssue)
			priorityReleaseDate := a.getNearestFixVersionDate(priorityIssue)
			if (activeIssue.Fields.Duedate == priorityIssue.Fields.Duedate) && (activeReleaseDate == priorityReleaseDate) ||
				(time.Time(activeIssue.Fields.Duedate).Before(time.Time(priorityIssue.Fields.Duedate)) || time.Time(priorityIssue.Fields.Duedate).IsZero()) &&
					(activeReleaseDate.Before(priorityReleaseDate) || len(priorityIssue.Fields.FixVersions) == 0) {
				continue
			}
		}
		//check active issues for last our, because hubstaff updates time estimate one time in hour
		activeIssueTimeStarted := *activeIssue.Fields.Worklog.Worklogs[0].Started
		if time.Time(activeIssueTimeStarted).UTC().Before(hourAgoUTC) {
			continue
		}
		var tl string
		switch {
		case common.ValueIn(user[TagUserSlackRealName], a.Slack.Employees.BeTeam...):
			tl = a.Slack.Employees.TeamLeaderBE
		case common.ValueIn(user[TagUserSlackRealName], a.Slack.Employees.FeTeam...):
			tl = a.Slack.Employees.TeamLeaderFE
		case common.ValueIn(user[TagUserSlackRealName], a.Slack.Employees.Design...):
			tl = a.Slack.Employees.ArtDirector
		case common.ValueIn(user[TagUserSlackRealName], a.Slack.Employees.DevOps...):
			tl = a.Slack.Employees.TeamLeaderDevOps
		}
		a.Slack.SendMessage(fmt.Sprintf("<@%s> начал работать над %s вперед %s \nfyi %s %s",
			user[TagUserSlackID], activeIssue.Link(), priorityIssue.Link(), a.Slack.Employees.ProjectManager, tl), channel)
	}
}

// ReportIssuesLockedByLowPriority report about issues locked by lower priority
func (a *App) ReportIssuesLockedByLowPriority(channel string) {
	issues, err := a.Jira.IssuesOfOpenSprints()
	if err != nil {
		return
	}
	msg := ""
	for _, issue := range issues {
		if issue.Fields.Status.Name == jira.StatusClosed {
			continue
		}
		for _, iLink := range issue.Fields.IssueLinks {
			if iLink.Type.Inward == jira.InwardIsBlockedBy && iLink.InwardIssue != nil {
				if iLink.InwardIssue.Fields.Status.Name == jira.StatusClosed {
					continue
				}
				// lower ID = higher priority
				if issue.Fields.Priority.ID < iLink.InwardIssue.Fields.Priority.ID {
					msg += fmt.Sprintf("%s (%s) заблокирована %s (%s)\n",
						issue.Link(), issue.Fields.Priority.Name,
						fmt.Sprintf("<%s/browse/%[2]s|%[2]s>", a.Config.Jira.APIUrl, iLink.InwardIssue.Key),
						iLink.InwardIssue.Fields.Priority.Name)
				}
			}
		}
	}
	a.Slack.SendMessage(msg, channel)
}

func (a *App) getNearestFixVersionDate(issue jira.Issue) time.Time {
	var releaseDate time.Time
	for _, version := range issue.Fields.FixVersions {
		if version.Name == "" {
			continue
		}
		slice := strings.Split(version.Name, "/")
		if len(slice) != 2 {
			continue
		}
		date, err := time.Parse("20060102", slice[1])
		if err != nil {
			logrus.WithError(err).Error("can't parse issue fix version start date")
			return time.Time{}
		}
		if releaseDate.IsZero() || date.Before(releaseDate) {
			releaseDate = date
		}
	}
	return releaseDate
}

// CheckNeedReplyMessages check messages in all channels for need to reply on it if user was mentioned
func (a *App) CheckNeedReplyMessages() {
	latestUnix := time.Now().Add(-12 * time.Hour).Unix()
	oldestUnix := time.Now().Add(-11 * time.Hour).Unix()
	channelsList, err := a.Slack.ChannelsList()
	if err != nil {
		logrus.WithError(err).Error("Can not get channels list")
		return
	}
	for _, channel := range channelsList {
		if !channel.IsChannelActual() {
			continue
		}
		channel.RemoveBotMembers(a.Config.BotIDs...)
		channelMessages, err := a.Slack.ChannelMessageHistory(channel.ID, oldestUnix, latestUnix)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"channelID": channel.ID, "latestUnix": latestUnix, "oldestUnix": oldestUnix}).Error("Can not get messages from channel")
			return
		}
		for _, channelMessage := range channelMessages {
			repliedUsers := channelMessage.RepliedUsers()
			var replyMessages []slack.Message
			// check for replies of channel message
			for _, reply := range channelMessage.Replies {
				replyMessage, err := a.Slack.ChannelMessage(channel.ID, reply.Ts)
				if err != nil {
					logrus.WithError(err).WithFields(logrus.Fields{"channelID": channel.ID, "ts": reply.Ts}).Error("Can not get reply for message from channel")
					return
				}
				if replyMessage.IsMessageFromBot() {
					continue
				}
				replyMessages = append(replyMessages, replyMessage)
			}
			// check reactions of channel members on message if it contains @channel
			if strings.Contains(channelMessage.Text, "<!channel>") {
				reactedUsers := channelMessage.ReactedUsers()
				var notReactedUsers []string
				for _, member := range channel.Members {
					if !common.ValueIn(member, reactedUsers...) && !common.ValueIn(member, repliedUsers...) && member != channelMessage.User {
						notReactedUsers = append(notReactedUsers, member)
					}
				}
				if len(notReactedUsers) == 0 {
					continue
				}
				var message string
				for _, userID := range notReactedUsers {
					message += "<@" + userID + "> "
				}
				a.Slack.SendToThread(message+" ^", channel.ID, channelMessage.Ts)
			}
			var mentionedUsers = make(map[string]string)
			if !channelMessage.IsMessageFromBot() {
				reactedUsers := channelMessage.ReactedUsers()
				for _, userSlackID := range channel.Members {
					if strings.Contains(channelMessage.Text, userSlackID) && mentionedUsers[userSlackID] == "" && !common.ValueIn(userSlackID, reactedUsers...) {
						mentionedUsers[userSlackID] = channelMessage.Ts
					}
				}
			}
			// send mention if ReplyCount = 0
			if channelMessage.ReplyCount == 0 {
				var message string
				if len(mentionedUsers) == 0 {
					continue
				}
				for userID := range mentionedUsers {
					message += "<@" + userID + "> "
				}
				messagePermalink, err := a.Slack.MessagePermalink(channel.ID, channelMessage.Ts)
				if err != nil {
					logrus.WithError(err).WithFields(logrus.Fields{"channelID": channel.ID, "ts": channelMessage.Ts}).Error("Can not get permalink for message from channel")
					return
				}
				a.Slack.SendToThread(fmt.Sprintf("%s %s", message, messagePermalink), channel.ID, channelMessage.Ts)
				continue
			}
			// check replies for message and new nemtions in replies
			for _, replyMessage := range replyMessages {
				delete(mentionedUsers, replyMessage.User)
				if channelMessage.IsMessageFromBot() {
					continue
				}
				// if users reacted we don't send message
				reactedUsers := replyMessage.ReactedUsers()
				for _, userSlackID := range channel.Members {
					if strings.Contains(replyMessage.Text, userSlackID) && mentionedUsers[userSlackID] == "" && !common.ValueIn(userSlackID, reactedUsers...) {
						mentionedUsers[userSlackID] = replyMessage.Ts
					}
				}
			}
			for userID, replyTs := range mentionedUsers {
				replyPermalink, err := a.Slack.MessagePermalink(channel.ID, replyTs)
				if err != nil {
					logrus.WithError(err).WithFields(logrus.Fields{"channelID": channel.ID, "ts": replyTs}).Error("Can not get permalink for message from channel")
					return
				}
				a.Slack.SendToThread(fmt.Sprintf("<@%s> %s", userID, replyPermalink), channel.ID, channelMessage.Ts)
			}
		}
	}
}

// SendJiraMention sends message to DM in slack for mentioned users
func (a *App) SendJiraMention(comment jira.Comment, issue jira.Issue) {
	if !strings.Contains(comment.Body, "[~accountid:") {
		return
	}
	ids := getUniqueJiraAccountIDsFromText(comment.Body)
	for _, id := range ids {
		slackID := a.GetUserInfoByTagValue(TagUserJiraAccountID, id)[TagUserSlackID]
		if slackID == "" {
			logrus.WithField("jiraAccountID", id).Error("Can't find slack id for user from jira")
			continue
		}
		a.Slack.SendMessage("Вас упомянули в комментарии к задаче:\n"+issue.String(), slackID)
	}
}

// getUniqueJiraAccountIDsFromText returns unique ids of mentioned users in jira issue comment text
func getUniqueJiraAccountIDsFromText(text string) []string {
	accountIDs := make([]string, 0)
	r, err := regexp.Compile(`(\[~accountid):[\w]*:*[\w]*-*[\w]*-*[\w]*-*[\w]*-*[\w]*]`)
	if err != nil {
		logrus.WithError(err).Error("Can't compile regexp")
		return []string{}
	}
	accountIDs = r.FindAllString(text, -1)
	for i := 0; i < len(accountIDs); i += 1 {
		accountIDs[i] = strings.TrimLeft(strings.TrimRight(accountIDs[i], "]"), "[~accountid:")
	}
	accountIDs = common.RemoveDuplicates(accountIDs)
	return accountIDs
}
