package app

import (
	"bytes"
	"context"
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

	"backoffice_app/app/reports"
	"backoffice_app/app/tg_bot"
	"backoffice_app/common"
	"backoffice_app/config"
	"backoffice_app/model"
	"backoffice_app/services/bitbucket"
	"backoffice_app/services/hubstaff"
	"backoffice_app/services/jira"
	"backoffice_app/services/slack"
	"backoffice_app/types"

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
	Hubstaff   hubstaff.Hubstaff
	Slack      slack.Slack
	Jira       jira.Jira
	Bitbucket  bitbucket.Bitbucket
	Config     config.Main
	AfkTimer   AfkTimer
	model      *model.Model
	ReleaseBot tg_bot.ReleaseBot
	Reports    struct {
		ForgottenBranches     reports.ForgottenBranches
		ForgottenPoolRequests reports.ForgottenPullRequests
	}
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
func New(conf *config.Main, m *model.Model, ctx context.Context, wg *sync.WaitGroup) *App {
	var (
		j = jira.New(&conf.Jira)
		b = bitbucket.New(&conf.Bitbucket)
		s = slack.New(&conf.Slack)
	)
	return &App{
		Hubstaff:   hubstaff.New(&conf.Hubstaff),
		Slack:      s,
		Jira:       j,
		ReleaseBot: tg_bot.NewReleaseBot(ctx, wg, conf.Telegram.ReleaseBotAPIKey, m, &j),
		Bitbucket:  b,
		Config:     *conf,
		AfkTimer:   AfkTimer{Mutex: &sync.Mutex{}, UserDurationMap: make(map[string]time.Duration)},
		model:      m,
		Reports: struct {
			ForgottenBranches     reports.ForgottenBranches
			ForgottenPoolRequests reports.ForgottenPullRequests
		}{
			ForgottenBranches:     reports.NewReportForgottenBranches(*m, b, *conf, s),
			ForgottenPoolRequests: reports.NewReportForgottenPullRequests(*m, b, *conf, s),
		},
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
			a.Jira.IssueSetStatusTransition(issue.Key, jira.StatusCloseLastTask)
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
				message += fmt.Sprintf("<https://atnr.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_%[4]s\n",
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
		return
	}

	ansibleCommits, err := a.model.GetCommitsByType(common.CommitTypeAnsible)
	if err != nil {
		return
	}
	// if commits cache is not empty return
	if len(migrationCommits) != 0 && len(ansibleCommits) != 0 {
		return
	}
	commits, err := a.Bitbucket.CommitsOfOpenedPRs()
	if err != nil {
		return
	}

	if len(migrationCommits) == 0 {
		SQLCommits, err := a.SQLCommitsCache(commits)
		if err != nil {
			return
		}
		err = a.CreateCommitsCache(SQLCommits)
		if err != nil {
			return
		}
	}

	if len(ansibleCommits) == 0 {
		AnsibleCommits, err := a.AnsibleCommitsCache(commits)
		if err != nil {
			return
		}

		a.CreateCommitsCache(AnsibleCommits)
	}
}

// MigrationMessages returns slice of all miigration files
func (a *App) MigrationMessages() ([]string, error) {
	commits, err := a.Bitbucket.CommitsOfOpenedPRs()
	if err != nil {
		return []string{}, err
	}

	newCommitsCache, err := a.SQLCommitsCache(commits)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, commit := range newCommitsCache {
		_, err := a.model.GetCommitByHash(commit.Hash)
		if err == common.ErrModelNotFound {
			file, err := a.Bitbucket.SrcFile(commit.Repository, commit.Hash, commit.Path)
			if err != nil {
				return []string{}, err
			}
			files = append(files, commit.Message+"\n```"+file+"```\n")
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	a.model.DeleteCommitsByType(common.CommitTypeMigration)
	a.CreateCommitsCache(newCommitsCache)
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
			usersAtWork += fmt.Sprintf(" <https://atnr.atlassian.net/browse/%[1]s|%[1]s - %[2]s>",
				activity.TaskJiraKey, activity.TaskSummary)
		}
		note, err := a.Hubstaff.LastUserNote(strconv.Itoa(activity.User.ID), strconv.Itoa(activity.LastProjectID))
		if err != nil {
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
		return err
	}
	issuesWithClosedSubtasks, err := a.Jira.IssuesClosedSubtasksFromOpenSprint(project)
	if err != nil {
		return err
	}
	issuesForNextSprint, err := a.Jira.IssuesForNextSprint(project)
	if err != nil {
		return err
	}
	issuesFromFutureSprint, err := a.Jira.IssuesFromFutureSprint(project)
	if err != nil {
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
		return err
	}
	sprintInterface, ok := issuesBugStoryOfOpenSprint[0].Fields.Unknowns[jira.FieldSprintInfo].([]interface{})
	if !ok {
		logrus.WithFields(logrus.Fields{"project": project, "channel": channel}).
			Errorf("can't parse to interface: %v", issuesWithClosedSubtasks[0].Fields.Unknowns[jira.FieldSprintInfo])
		return common.ErrInternal
	}
	sprintSequence, err := a.FindLastSprintSequence(sprintInterface)
	if err != nil {
		return err
	}
	for _, issue := range issuesFromFutureSprint {
		issuesForNextSprint = append(issuesForNextSprint, issue)
	}
	err = a.CreateIssuesCsvReport(issuesForNextSprint, fmt.Sprintf("Sprint %v Open", sprintSequence), channel, false)
	if err != nil {
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
		logrus.WithError(err).WithFields(logrus.Fields{"issues": issues, "filename": filename, "channel": channel, "withAdditionalInfo": withAdditionalInfo}).
			Error("can't create file")
		return common.ErrInternal
	}
	logFields := logrus.Fields{
		"issues":             issues,
		"filename":           filename,
		"channel":            channel,
		"withAdditionalInfo": withAdditionalInfo,
		"raw":                []string{},
	}
	writer := csv.NewWriter(file)
	if withAdditionalInfo {
		err = writer.Write([]string{"Type", "Key", "Summary", "Status", "Epic"})
		if err != nil {
			logFields["raw"] = []string{"Type", "Key", "Summary", "Status", "Epic"}
			logrus.WithError(err).WithFields(logFields).Error("can't create raw in csv")
			return common.ErrInternal
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
				logFields["raw"] = []string{issue.Fields.Type.Name, issue.Key, issue.Fields.Summary, issue.Fields.Status.Name, epicName}
				logrus.WithError(err).WithFields(logFields).Error("can't create raw in csv")
				return common.ErrInternal
			}
		}
	}
	if !withAdditionalInfo {
		err = writer.Write([]string{"Type", "Key", "Summary"})
		if err != nil {
			logFields["raw"] = []string{"Type", "Key", "Summary"}
			logrus.WithError(err).WithFields(logFields).Error("can't create raw in csv")
			return common.ErrInternal
		}
		for _, issue := range issues {
			err = writer.Write([]string{issue.Fields.Type.Name, issue.Key, issue.Fields.Summary})
			if err != nil {
				logFields["raw"] = []string{issue.Fields.Type.Name, issue.Key, issue.Fields.Summary}
				logrus.WithError(err).WithFields(logFields).Error("can't create raw in csv")
				return common.ErrInternal
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
		logrus.WithError(err).WithFields(logrus.Fields{"regexp": `sequence=(\d+)`, "sprints": sprints}).Error("can't find by regexp")
		return 0, common.ErrInternal
	}
	for i := range sprints {
		s, ok := sprints[i].(string)
		if !ok {
			logrus.WithField("sprints", sprints).Errorf("can't find sprint of closed subtasks")
			return 0, common.ErrInternal
		}
		// Find string submatch and get slice of match string and this sequence
		// For example, one of sprint:
		// "com.atlassian.greenhopper.service.sprint.Sprint@6f00eb7b[id=47,rapidViewId=12,state=ACTIVE,name=Sprint 46,
		// goal=,startDate=2019-02-20T04:19:23.907Z,endDate=2019-02-25T04:19:00.000Z,completeDate=<null>,sequence=47]"
		// we get string submatch of slice ["sequence=47" "47"] and then parse "47" as integer number to find the biggest one
		m := rSeq.FindStringSubmatch(s)
		if len(m) != 2 {
			logrus.WithField("sprints", sprints).Errorf("can't find submatch string to sequence: %v", sprints[i])
			return 0, common.ErrInternal
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"value": m[1], "sprints": sprints}).Error("can't parse to int")
			return 0, common.ErrInternal
		}
		if n > lastSequence {
			lastSequence = n
		}
	}
	return lastSequence, nil
}

// SendFileToSlack sends file to slack
// TODO: remove duplicate func
// DEPRECATED: use some function from slack package
func (a *App) SendFileToSlack(channel, fileName string) error {
	fileDir, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"filename": fileName,
			"channel":  channel,
		}).Error("can't find file")
		return common.ErrInternal
	}
	filePath := path.Join(fileDir, fileName)
	file, err := os.Open(filePath)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"filename": fileName,
			"channel":  channel,
		}).Error("can't open file")
		return common.ErrInternal
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"filename": fileName,
			"channel":  channel,
		}).Error("can't create multipart from file file")
		return common.ErrInternal
	}
	_, err = io.Copy(part, file)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"filename": fileName,
			"channel":  channel,
		}).Error("can't copy multipart from file")
		return common.ErrInternal
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
		return
	}
	sprintInterface, ok := openIssues[0].Fields.Unknowns[jira.FieldSprintInfo].([]interface{})
	if !ok {
		logrus.WithError(err).WithField("map", openIssues[0].Fields.Unknowns[jira.FieldSprintInfo]).Error("can't parse interface from map")
		return
	}
	startDate, endDate, err := a.FindLastSprintDates(sprintInterface)
	if err != nil {
		return
	}
	closedIssues, err := a.Jira.IssuesClosedInInterim(startDate.AddDate(0, 0, -1), endDate.AddDate(0, 0, +1))
	if err != nil {
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
					message += fmt.Sprintf("<https://atnr.atlassian.net/browse/%[1]s|%[1]s> / ", issue.Fields.Parent.Key)
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
				logrus.WithField("accountID", accountID).Error("can't take user id by accountID from vocabulary")
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
		logrus.WithFields(logrus.Fields{
			"userName": userName,
			"date":     date,
			"channel":  channel,
		}).Error("Can't find user's data in vocabulary")
		return common.ErrInternal
	}
	layout := "2006-01-02"
	t, err := time.Parse(layout, date)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"userName": userName,
			"date":     date,
			"channel":  channel,
		}).Error("Can't parse date")
		return common.ErrInternal
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
		return
	}
	newAnsibleCache, err := a.AnsibleCommitsCache(commits)
	if err != nil {
		return
	}
	var files []string
	for _, commit := range newAnsibleCache {
		_, err := a.model.GetCommitByHash(commit.Hash)
		if err == common.ErrModelNotFound {
			file, err := a.Bitbucket.SrcFile(commit.Repository, commit.Hash, commit.Path)
			if err != nil {
				return
			}
			files = append(files, commit.Message+"\n```"+file+"```\n")
			continue
		}
		if err != nil {
			return
		}
	}
	a.model.DeleteCommitsByType(common.CommitTypeAnsible)
	a.CreateCommitsCache(newAnsibleCache)
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
func (a *App) StartAfkTimer(userDuration time.Duration, userID string) {
	a.model.CreateAfkTimer(model.AfkTimer{UserID: userID, Duration: userDuration.String()})
	a.AfkTimer.UserDurationMap[userID] = userDuration
	ticker := time.NewTicker(time.Second)
	go func() {
		for range ticker.C {
			a.AfkTimer.Lock()
			a.AfkTimer.UserDurationMap[userID] = a.AfkTimer.UserDurationMap[userID] - time.Second
			a.AfkTimer.Unlock()
			if a.AfkTimer.UserDurationMap[userID] <= 0 {
				ticker.Stop()
				a.model.DeleteAfkTimer(userID)
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
		return
	}
	for _, vacation := range vacations {
		if strings.Contains(message, vacation.UserID) {
			userInfo := a.GetUserInfoByTagValue(TagUserSlackID, vacation.UserID)
			if userInfo[TagUserSlackRealName] == "" || userInfo[TagUserSlackRealName] == EmptyTagValue {
				logrus.Errorf("can't take information about user name from vocabulary with id: %v", vacation.UserID)
				userInfo[TagUserSlackRealName] = "This user"
			}
			a.Slack.SendToThread(fmt.Sprintf("*%s* is on vacation until %s, his message is: \n\n'%s'",
				userInfo[TagUserSlackRealName], vacation.DateEnd.Format("02.01.2006"), vacation.Message), channel, threadId)
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
		return
	}
	if reviewCount < 2 {
		return
	}
	developerID := issue.DeveloperMap(jira.TagDeveloperID)
	userInfo := a.GetUserInfoByTagValue(TagUserJiraAccountID, developerID)
	var userID string
	switch {
	case userInfo[TagUserSlackID] == EmptyTagValue:
		userID = userInfo[TagUserEmail]
	case userInfo[TagUserSlackID] == "":
		userID = jira.NoDeveloper
	default:
		userID = "<@" + userInfo[TagUserSlackID] + ">"
	}

	msgBody := fmt.Sprintf("The issue %s has been rejected after %v reviews\n\n", issue.Key, reviewCount)
	switch issue.Fields.Type.Name {
	case jira.TypeBESubTask, jira.TypeBETask:
		msgBody += fmt.Sprintf("Developer: %s\nfyi %s\nсс %s", userID, a.Slack.Employees.TeamLeaderBE, a.Slack.Employees.DirectorOfCompany)
	case jira.TypeFESubTask, jira.TypeFETask:
		msgBody += fmt.Sprintf("Developer: %s\nfyi %s\nсс %s", userID, a.Slack.Employees.TeamLeaderFE, a.Slack.Employees.DirectorOfCompany)
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
			go a.StartAfkTimer(duration-difference, afkTimer.UserID)
			continue
		}
		a.model.DeleteAfkTimer(afkTimer.UserID)
	}
}

// ReportOverworkedIssues create report about overworked issues
func (a *App) ReportOverworkedIssues(channel string) {
	issues, err := a.Jira.IssuesClosedInInterim(
		now.BeginningOfWeek().AddDate(0, 0, -8),
		now.EndOfWeek().AddDate(0, 0, -6))
	if err != nil {
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
		logrus.WithError(err).WithField("sprints", sprints).Error("can't compile regexp")
		return time.Time{}, time.Time{}, common.ErrInternal
	}
	eDate, err := regexp.Compile(`endDate=(\d{4}-\d{2}-\d{2})`)
	if err != nil {
		logrus.WithError(err).WithField("sprints", sprints).Error("can't compile regexp")
		return time.Time{}, time.Time{}, common.ErrInternal
	}
	for i := range sprints {
		s, ok := sprints[i].(string)
		if !ok {
			logrus.WithError(err).WithField("date", sprints[i]).Error("can't parse to string")
			return time.Time{}, time.Time{}, common.ErrInternal
		}
		// Find string submatch and get slice of match string and this startDate
		// For example, one of sprint:
		// "com.atlassian.greenhopper.service.sprint.Sprint@6f00eb7b[id=47,rapidViewId=12,state=ACTIVE,name=Sprint 46,
		// goal=,startDate=2019-02-20T04:19:23.907Z,endDate=2019-02-25T04:19:00.000Z,completeDate=<null>,sequence=47]"
		// we get string submatch of slice ["startDate=2019-02-20" "2019-02-20"] and then parse "2019-02-20" as time to find the biggest one
		sd := sDate.FindStringSubmatch(s)
		if len(sd) != 2 {
			logrus.WithError(err).WithField("startDate", sprints[i]).Error("can't find submatch string to start date")
			return time.Time{}, time.Time{}, common.ErrInternal
		}
		ts, err := time.Parse("2006-01-02", sd[1])
		if err != nil {
			logrus.WithError(err).WithField("date", sd[1]).Error("can't parse date")
			return time.Time{}, time.Time{}, common.ErrInternal
		}

		ed := eDate.FindStringSubmatch(s)
		if len(ed) != 2 {
			logrus.WithError(err).WithField("endDate", sprints[i]).Error("can't find submatch string to end date")
			return time.Time{}, time.Time{}, common.ErrInternal
		}
		te, err := time.Parse("2006-01-02", ed[1])
		if err != nil {
			logrus.WithError(err).WithField("date", sd[1]).Error("can't parse date")
			return time.Time{}, time.Time{}, common.ErrInternal
		}

		if ts.After(startDate) {
			startDate = ts
			endDate = te
		}
	}
	return startDate, endDate, nil
}

// SetVacationPeriod create vacation period for user
func (a *App) SetVacationPeriod(dateStart, dateEnd, message, userID string) error {
	dStart, err := time.Parse("02.01.2006", dateStart)
	if err != nil {
		logrus.WithError(err).WithField("dateStart", dateStart).Error("can't parse start date")
		return common.ErrInternal
	}
	dEnd, err := time.Parse("02.01.2006", dateEnd)
	if err != nil {
		logrus.WithError(err).WithField("dateEnd", dateEnd).Error("can't parse end date")
		return common.ErrInternal
	}
	if dStart.After(dEnd) {
		logrus.WithFields(logrus.Fields{"dateStart": dateStart, "dateEnd": dateEnd}).Error("Date of start vacation bigger then data of end")
		return common.ErrConflict{"Date of start vacation bigger then data of end"}
	}

	if err := a.model.SaveVacation(model.Vacation{
		UserID:    userID,
		DateStart: dStart,
		DateEnd:   dEnd,
		Message:   message,
	}); err != nil {
		return err
	}
	return nil
}

// CancelVacation delete vacation
func (a *App) CancelVacation(userID string) error {
	_, err := a.CheckVacationSatus(userID)
	if err != nil {
		return err
	}
	err = a.model.DeleteVacation(userID)
	if err != nil {
		return err
	}
	return nil
}

// CheckVacationSatus get vacation if exist
func (a *App) CheckVacationSatus(userID string) (model.Vacation, error) {
	vacation, err := a.model.GetVacation(userID)
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
		a.Bitbucket.CreateBranch(issue.Key, issue.Key, "master")
		return
	}
	err := a.Bitbucket.CreateBranch(issue.Key, issue.Fields.Parent.Key, "master")
	if err != nil {
		return
	}
	a.Bitbucket.CreateBranch(issue.Key, issue.Fields.Parent.Key+">"+issue.Key, issue.Fields.Parent.Key)
}

// CreateBranchPullRequest create pull request for first branch commit
func (a *App) CreateBranchPullRequest(repoPushPayload bitbucket.RepoPushPayload) {
	// if commit was deleted or branch was deleted, new name will be empty, and we check it to do nothing
	if repoPushPayload.Push.Changes[0].New.Name == "" {
		return
	}
	if !strings.Contains(repoPushPayload.Push.Changes[0].New.Name, ">") {
		a.Bitbucket.CreatePullRequestIfNotExist(repoPushPayload.Repository.Name, repoPushPayload.Push.Changes[0].New.Name, "master")
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
		return
	}
}

// ReportEpicsWithClosedIssues create report about epics with closed issues
func (a *App) ReportEpicsWithClosedIssues(channel string) {
	epics, err := a.Jira.EpicsWithClosedIssues()
	if err != nil {
		return
	}
	if len(epics) == 0 {
		a.Slack.SendMessage("There are no epics with all closed issues", channel)
		return
	}
	var msgBody string
	for _, epic := range epics {
		if epic.Fields.Status.Name != jira.StatusInArtDirectorReview {
			a.Jira.IssueSetStatusTransition(epic.Key, jira.StatusInArtDirectorReview)
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
		return
	}
	var authorPullRequests = make(map[string][]string)
	for _, pullRequest := range pullRequests {
		diff, err := a.Bitbucket.PullRequestDiff(pullRequestPayload.Repository.Name, pullRequest.ID)
		if err != nil {
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
// DEPRECATED: use config.GetUserInfoByTagValue instead
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
	if msg != "" {
		a.Slack.SendMessage(msg, channel)
	}
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
		return
	}
	for _, channel := range channelsList {
		if !channel.IsChannelActual() {
			continue
		}
		channel.RemoveBotMembers(a.Config.BotIDs...)
		channelMessages, err := a.Slack.ChannelMessageHistory(channel.ID, oldestUnix, latestUnix)
		if err != nil {
			return
		}
		for _, channelMessage := range channelMessages {
			repliedUsers := channelMessage.RepliedUsers()
			var replyMessages []slack.Message
			// check for replies of channel message
			for _, reply := range channelMessage.Replies {
				replyMessage, err := a.Slack.ChannelMessage(channel.ID, reply.Ts)
				if err != nil {
					return
				}
				if replyMessage.IsMessageFromBot() {
					continue
				}
				replyMessages = append(replyMessages, replyMessage)
			}
			// check reactions of channel members on message if it contains @channel
			if strings.Contains(channelMessage.Text, "<!channel>") {
				a.sendMessageToNotReactedUsers(channelMessage, channel, repliedUsers)
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
			a.sendMessageToMentionedUsers(channelMessage, channel, mentionedUsers)
		}
	}
}

// sendMessageToNotReactedUsers sends messages to not reacted users for CheckNeedReplyMessages method
func (a *App) sendMessageToNotReactedUsers(channelMessage slack.Message, channel slack.Channel, repliedUsers []string) {
	reactedUsers := channelMessage.ReactedUsers()
	var notReactedUsers []string
	for _, member := range channel.Members {
		if !common.ValueIn(member, reactedUsers...) && !common.ValueIn(member, repliedUsers...) && member != channelMessage.User {
			notReactedUsers = append(notReactedUsers, member)
		}
	}
	if len(notReactedUsers) == 0 {
		return
	}
	afkUsers, err := a.getAfkUsersIDs()
	if err != nil {
		return
	}
	var message string
	for _, userID := range notReactedUsers {
		if common.ValueIn(userID, afkUsers...) {
			a.model.CreateReminder(model.Reminder{
				UserID:     userID,
				Message:    "<@" + userID + "> ",
				ChannelID:  channel.ID,
				ThreadTs:   channelMessage.Ts,
				ReplyCount: channelMessage.ReplyCount,
			})
			continue
		}
		message += "<@" + userID + "> "
	}
	a.Slack.SendToThread(message+" ^", channel.ID, channelMessage.Ts)
}

// sendMessageToMentionedUsers sends messages to mentioned users for CheckNeedReplyMessages method
func (a *App) sendMessageToMentionedUsers(channelMessage slack.Message, channel slack.Channel, mentionedUsers map[string]string) {
	if len(mentionedUsers) == 0 {
		return
	}
	afkUsers, err := a.getAfkUsersIDs()
	if err != nil {
		return
	}
	messages := make(map[string]string)
	for userID, replyTs := range mentionedUsers {
		replyPermalink, err := a.Slack.MessagePermalink(channel.ID, replyTs)
		if err != nil {
			return
		}
		if common.ValueIn(userID, afkUsers...) {
			a.model.CreateReminder(model.Reminder{
				UserID:     userID,
				Message:    fmt.Sprintf("<@%s> %s", userID, replyPermalink),
				ChannelID:  channel.ID,
				ThreadTs:   channelMessage.Ts,
				ReplyCount: channelMessage.ReplyCount,
			})
			continue
		}
		messages[replyPermalink] += "<@" + userID + "> "
	}
	for messagePermalink, message := range messages {
		a.Slack.SendToThread(fmt.Sprintf("%s %s", message, messagePermalink), channel.ID, channelMessage.Ts)
	}
}

// getAfkUsersIDs retrieves all afk users on vacation or with afk status
func (a *App) getAfkUsersIDs() ([]string, error) {
	var usersIDs []string
	afkTimers, err := a.model.GetAfkTimers()
	if err != nil {
		return []string{}, err
	}
	for _, at := range afkTimers {
		usersIDs = append(usersIDs, at.UserID)
	}
	vacations, err := a.model.GetActualVacations()
	if err != nil {
		return []string{}, err
	}
	for _, v := range vacations {
		if common.ValueIn(v.UserID, usersIDs...) {
			continue
		}
		usersIDs = append(usersIDs, v.UserID)
	}
	return usersIDs, nil
}

// SendReminders sends reminders for non afk users
func (a *App) SendReminders() {
	afkUsers, err := a.getAfkUsersIDs()
	if err != nil {
		return
	}
	reminders, err := a.model.GetReminders()
	if err != nil {
		return
	}
	for _, reminder := range reminders {
		if common.ValueIn(reminder.UserID, afkUsers...) {
			continue
		}
		message, err := a.Slack.ChannelMessage(reminder.ChannelID, reminder.ThreadTs)
		if err != nil {
			return
		}
		newReplies := message.Replies[reminder.ReplyCount-1:]
		var wasAnswered bool
		for _, reply := range newReplies {
			if reply.User != reminder.UserID {
				continue
			}
			wasAnswered = true
			break
		}
		if !wasAnswered {
			a.Slack.SendToThread(reminder.Message, reminder.ChannelID, reminder.ThreadTs)
		}
		if err := a.model.DeleteReminder(reminder.ID); err != nil {
			return
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
		a.Slack.SendMessage("Вас упомянули в комментарии к задаче:\n"+issue.LinkWithDescription(), slackID)
	}
}

// getUniqueJiraAccountIDsFromText returns unique ids of mentioned users in jira issue comment text
func getUniqueJiraAccountIDsFromText(text string) []string {
	accountIDs := make([]string, 0)
	r, err := regexp.Compile(`(\[~accountid):[\w]*:*[\w]*-*[\w]*-*[\w]*-*[\w]*-*[\w]*]`)
	if err != nil {
		logrus.WithError(err).WithField("regexp", `(\[~accountid):[\w]*:*[\w]*-*[\w]*-*[\w]*-*[\w]*-*[\w]*]`).Error("Can't compile regexp")
		return []string{}
	}
	accountIDs = r.FindAllString(text, -1)
	for i := 0; i < len(accountIDs); i++ {
		accountIDs[i] = strings.TrimLeft(strings.TrimRight(accountIDs[i], "]"), "[~accountid:")
	}
	accountIDs = common.RemoveDuplicates(accountIDs)
	return accountIDs
}

func (a *App) SetOnDutyUsers(team string, userMentions []string) error {
	vacations, err := a.model.GetActualVacations()
	if err != nil {
		return err
	}
	var usersIDsOnVacation []string
	for _, vacation := range vacations {
		usersIDsOnVacation = append(usersIDsOnVacation, vacation.UserID)
	}
	users, err := a.Slack.UsersSlice()
	if err != nil {
		return err
	}
	var usersOnDuty []slack.Member
	for _, user := range users {
		if !common.ValueIn("@"+user.Name, userMentions...) {
			continue
		}
		if common.ValueIn(user.Id, usersIDsOnVacation...) {
			return common.ErrConflict{fmt.Sprintf("User <@%s> is on vacation!", user.Id)}
		}
		usersOnDuty = append(usersOnDuty, user)
	}
	if len(usersOnDuty) == 0 {
		logrus.WithField("userMentions", userMentions).Error("There aren't real users for adding on duty")
		return common.ErrConflict{"There aren't real users for adding on duty"}
	}
	tx, err := a.model.StartTransaction()
	if err != nil {
		return err
	}
	if err := tx.DeleteOnDutyUsersByTeam(team); err != nil {
		tx.RollBackTransaction()
		return err
	}
	for _, user := range usersOnDuty {
		if err := tx.CreateOnDutyUser(model.OnDutyUser{Team: team, SlackUserID: user.Id}); err != nil {
			tx.RollBackTransaction()
			return err
		}
	}
	return tx.CommitTransaction()
}

func (a *App) SendMentionUsersOnDuty(message, ts, channel string) {
	if strings.Contains(strings.ToLower(message), common.OnDutyBe) {
		users, err := a.model.GetOnDutyUsersByTeam(common.DevTeamBackend)
		if err != nil {
			return
		}
		var message string
		for _, user := range users {
			message += "<@" + user.SlackUserID + "> "
		}
		a.Slack.SendToThread(message+"^", channel, ts)
	}
	if strings.Contains(strings.ToLower(message), common.OnDutyFe) {
		users, err := a.model.GetOnDutyUsersByTeam(common.DevTeamFrontend)
		if err != nil {
			return
		}
		var message string
		for _, user := range users {
			message += "<@" + user.SlackUserID + "> "
		}
		a.Slack.SendToThread(message+"^", channel, ts)
	}
}

func (a *App) SendMentionUsersInTeam(message, ts, channel string) {
	if strings.Contains(strings.ToLower(message), common.BETeam) {
		var message string
		for _, member := range a.Config.Slack.Employees.BeTeam {
			userSlackID := a.GetUserInfoByTagValue(TagUserSlackRealName, member)[TagUserSlackID]
			if userSlackID == "" {
				continue
			}
			message += "<@" + userSlackID + "> "
		}
		a.Slack.SendToThread(message+" "+a.Slack.Employees.TeamLeaderBE, channel, ts)
	}
	if strings.Contains(strings.ToLower(message), common.FETeam) {
		var message string
		for _, member := range a.Config.Slack.Employees.FeTeam {
			userSlackID := a.GetUserInfoByTagValue(TagUserSlackRealName, member)[TagUserSlackID]
			if userSlackID == "" {
				continue
			}
			message += "<@" + userSlackID + "> "
		}
		a.Slack.SendToThread(message+" "+a.Slack.Employees.TeamLeaderFE, channel, ts)
	}
	if strings.Contains(strings.ToLower(message), common.QATeam) {
		var message string
		for _, member := range a.Config.Slack.Employees.QATeam {
			userSlackID := a.GetUserInfoByTagValue(TagUserSlackRealName, member)[TagUserSlackID]
			if userSlackID == "" {
				continue
			}
			message += "<@" + userSlackID + "> "
		}
		a.Slack.SendToThread(message, channel, ts)
	}
}
