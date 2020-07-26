// Notify about almost closed issues.
// it may be unassigned, therefore be lost without release
// the main goal is prevent lost issues
package reports

import (
	"backoffice_app/services/jira"
	"backoffice_app/services/slack"
)

type ClosedSubtasks struct {
	jira  jira.Jira
	slack slack.Slack
}

func NewClosedSubtasks(
	j jira.Jira,
	s slack.Slack,
) ClosedSubtasks {
	return ClosedSubtasks{
		jira:  j,
		slack: s,
	}
}

// ReportIsuuesWithClosedSubtasks create report about issues with closed subtasks
func (a *ClosedSubtasks) Run() {
	issues, err := a.jira.IssuesWithClosedSubtasks()
	if err != nil {
		return
	}
	for i := len(issues) - 1; i > 0; i-- {
		if issues[i].Fields.Status.Name == jira.StatusReadyForRelease {
			issues = append(issues[:i], issues[i+1:]...)
		}
	}
	if len(issues) == 0 {
		return
	}

	var message = "*Issues have all closed subtasks:*\n\n"
	for _, issue := range issues {
		message += issue.String()
	}
	a.slack.SendMessage(message, "#back-office-app")
}
