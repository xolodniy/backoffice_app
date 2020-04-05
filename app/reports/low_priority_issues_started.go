// report if somebody starts working on low-priority issue and has more priority issue at the same time
package reports

import (
	"fmt"
	"strings"
	"time"

	"backoffice_app/common"
	"backoffice_app/config"
	"backoffice_app/services/jira"
	"backoffice_app/services/slack"

	"github.com/sirupsen/logrus"
)

type LowPriorityIssuesStarted struct {
	config config.Main
	jira   jira.Jira
	slack  slack.Slack
}

func NewLowPriorityIssuesStarted(
	c config.Main,
	j jira.Jira,
	s slack.Slack,
) LowPriorityIssuesStarted {
	return LowPriorityIssuesStarted{
		config: c,
		jira:   j,
		slack:  s,
	}
}

// ReportLowPriorityIssuesStarted checks if developer start issue with low priority and send report about it
func (l LowPriorityIssuesStarted) Run(channel string) {
	// get all opened and started issues with one last worklog activity, sorted by priority from highest
	issues, err := l.jira.OpenedIssuesWithLastWorklogActivity()
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
		// TODO: add constant variable
		user := l.config.GetUserInfoByTagValue("jiraaccountid", developer)
		// check developers in ignore list
		if common.ValueIn(user["slackrealname"], l.config.IgnoreList...) {
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
				l.jira.IssueSetStatusTransition(issue.Key, jira.TransitionStart)
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
			activeReleaseDate := l.getNearestFixVersionDate(activeIssue)
			priorityReleaseDate := l.getNearestFixVersionDate(priorityIssue)
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
		case common.ValueIn(user["slackrealname"], l.slack.Employees.BeTeam...):
			tl = l.slack.Employees.TeamLeaderBE
		case common.ValueIn(user["slackrealname"], l.slack.Employees.FeTeam...):
			tl = l.slack.Employees.TeamLeaderFE
		case common.ValueIn(user["slackrealname"], l.slack.Employees.Design...):
			tl = l.slack.Employees.ArtDirector
		case common.ValueIn(user["slackrealname"], l.slack.Employees.DevOps...):
			tl = l.slack.Employees.TeamLeaderDevOps
		}
		l.slack.SendMessage(fmt.Sprintf("<@%s> начал работать над %s вперед %s \nfyi %s %s",
			user["slackid"], activeIssue.Link(), priorityIssue.Link(), l.slack.Employees.ProjectManager, tl), channel)
	}
}

func (l LowPriorityIssuesStarted) getNearestFixVersionDate(issue jira.Issue) time.Time {
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
