package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"backoffice_app/config"
	"backoffice_app/services/bitbucket"
	"backoffice_app/services/hubstaff"
	"backoffice_app/services/jira"
	"backoffice_app/services/slack"

	"github.com/sirupsen/logrus"
)

// App is main App implementation
type App struct {
	Hubstaff   hubstaff.Hubstaff
	Slack      slack.Slack
	Jira       jira.Jira
	Bitbucket  bitbucket.Bitbucket
	Config     config.Main
	SlackCache map[string][]map[int64]string
}

// New is main App constructor
func New(conf *config.Main) *App {
	return &App{
		Hubstaff:   hubstaff.New(&conf.Hubstaff),
		Slack:      slack.New(&conf.Slack),
		Jira:       jira.New(&conf.Jira),
		Bitbucket:  bitbucket.New(&conf.Bitbucket),
		Config:     *conf,
		SlackCache: make(map[string][]map[int64]string),
	}
}

// GetWorkersWorkedTimeAndSendToSlack gather workers work time made through period between dates and send it to Slack channel
func (a *App) GetWorkersWorkedTimeAndSendToSlack(prefix string, dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time, orgID int64) {
	orgsList, err := a.GetWorkersTimeByOrganization(dateOfWorkdaysStart, dateOfWorkdaysEnd, orgID)
	if err != nil {
		logrus.WithError(err).Error("can't get workers worked tim from Hubstaff")
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
		a.Slack.SendMessage("No tracked time for now or no organization found", a.Slack.ChanBackofficeApp)
		return
	}

	if len(orgsList[0].Workers) == 0 {
		message = "No tracked time for now or no workers found"
	} else {
		for _, worker := range orgsList[0].Workers {
			t, err := a.DurationString(worker.TimeWorked)
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

	a.Slack.SendMessage(message, a.Slack.ChanBackofficeApp)
}

// DurationString converts Seconds to 00:00 (hours with leading zero:minutes with leading zero) time format
func (a *App) DurationString(durationInSeconds int) (string, error) {
	var someTime time.Time
	r, err := regexp.Compile(` ([0-9]{2,2}:[0-9]{2,2}):[0-9]{2,2}`)
	if err != nil {
		return "", fmt.Errorf("regexp error: %v", err)
	}

	occurrences := r.FindStringSubmatch(someTime.Add(time.Second * time.Duration(durationInSeconds)).String())
	if len(occurrences) != 2 && &occurrences[1] == nil {
		return "", fmt.Errorf("no time after unix time parsing")
	}

	return occurrences[1], nil
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
		msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s>\n", issue.Key)
	}
	a.Slack.SendMessage(msgBody, a.Slack.ChanBackofficeApp)
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
		msgBody += fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s>\n", issue.Key)
	}
	a.Slack.SendMessage(msgBody, a.Slack.ChanBackofficeApp)
}

// ReportGitMigrations create report about new git migrations
func (a *App) ReportGitMigrations() {
	messages, err := a.MigrationMessages()
	if err != nil {
		logrus.WithError(err).Error("can't take information git migrations from bitbucket")
		return
	}
	if len(messages) == 0 {
		return
	}
	for _, message := range messages {
		a.Slack.SendMessage(message, a.Slack.ChanMigrations)
	}
}

// FillCache fill cache commits for searching new migrations
func (a *App) FillCache() {
	repositories, err := a.Bitbucket.RepositoriesList()
	if err != nil {
		logrus.Panic(err)
	}

	var allPullRequests bitbucket.PullRequests
	for _, repository := range repositories.Values {
		pullRequests, err := a.Bitbucket.PullRequestsList(repository.Name)
		if err != nil {
			logrus.Panic(err)
		}
		for _, pullRequest := range pullRequests.Values {
			if pullRequest.State == "OPEN" {
				allPullRequests.Values = append(allPullRequests.Values, pullRequest)
			}
		}
	}
	for _, pullRequest := range allPullRequests.Values {
		commits, err := a.Bitbucket.PullRequestCommits(pullRequest.Source.Repository.Name, strconv.FormatInt(pullRequest.ID, 10))
		if err != nil {
			logrus.Panic(err)
		}
		for _, commit := range commits.Values {
			a.SlackCache[pullRequest.Source.Repository.Name] = append(
				a.SlackCache[pullRequest.Source.Repository.Name],
				map[int64]string{pullRequest.ID: commit.Hash},
			)
		}
	}
}

//TODO убрать дублирование кода

// MigrationMessages returns slice of all miigration files
func (a *App) MigrationMessages() ([]string, error) {
	repositories, err := a.Bitbucket.RepositoriesList()
	if err != nil {
		return nil, err
	}

	var allPullRequests bitbucket.PullRequests
	for _, repository := range repositories.Values {
		pullRequests, err := a.Bitbucket.PullRequestsList(repository.Name)
		if err != nil {
			return nil, err
		}
		for _, pullRequest := range pullRequests.Values {
			if pullRequest.State == "OPEN" {
				allPullRequests.Values = append(allPullRequests.Values, pullRequest)
			}
		}
	}

	var files []string
	newCache := make(map[string][]map[int64]string)
	for _, pullRequest := range allPullRequests.Values {
		commits, err := a.Bitbucket.PullRequestCommits(pullRequest.Source.Repository.Name, strconv.FormatInt(pullRequest.ID, 10))
		if err != nil {
			logrus.Panic(err)
		}
		for _, commit := range commits.Values {
			diffStats, err := a.Bitbucket.CommitsDiffStats(commit.Repository.Name, commit.Hash)
			if err != nil {
				return nil, err
			}

			newCache[pullRequest.Source.Repository.Name] = append(
				newCache[pullRequest.Source.Repository.Name],
				map[int64]string{pullRequest.ID: commit.Hash},
			)
			func() {
				for _, commits := range a.SlackCache[commit.Repository.Name] {
					if commit.Hash == commits[pullRequest.ID] {
						return
					}
				}
				logrus.Debug("New!")
				for _, diffStat := range diffStats.Values {
					if strings.Contains(diffStat.New.Path, ".sql") {
						logrus.Debug(diffStat.New.Path)
						file, err := a.Bitbucket.SrcFile(commit.Repository.Name, commit.Hash, diffStat.New.Path)
						if err != nil {
							logrus.Panic(err)
						}
						files = append(files, pullRequest.Source.Branch.Name+"\n"+file)
					}
				}
			}()
		}
	}
	a.SlackCache = newCache
	return files, nil
}
