package jira

import (
	"fmt"

	"backoffice_app/config"

	"github.com/andygrunwald/go-jira"
	"github.com/sirupsen/logrus"
)

// Jira main struct of jira client
type Jira struct {
	*jira.Client
}

// Issue struct don't let go-jira dependency on App level
type Issue struct {
	jira.Issue
}

// New creates new jira
func New(config *config.Jira) Jira {
	jiraClient, err := jira.NewClient(config.Auth.Client(), config.APIUrl)
	if err != nil {
		panic(err)
	}
	return Jira{
		jiraClient,
	}
}

// Status variables for jql requests
var (
	StatusClosed          = "Closed"
	StatusTlReview        = "TL Review"
	StatusPeerReview      = "In peer review"
	StatusCloseLastTask   = "Close last task"
	StatusReadyForDemo    = "Ready for demo"
	StatusEmptyAssignee   = "empty"
	FieldEpicName         = "customfield_10005"
	FieldEpicKey          = "customfield_10008"
	FieldSprintInfo       = "customfield_10010"
	FieldDeveloperMap     = "customfield_10026"
	StatusInClarification = "In clarification"
)

// issues searches issues in all sprints which opened now and returning list with issues in this sprints list
func (j *Jira) issues(jqlRequest string) ([]Issue, error) {
	var issues []Issue
	for i := 0; ; i += 100 {
		allIssues, resp, err := j.Issue.Search(
			jqlRequest,
			&jira.SearchOptions{
				StartAt:    i,
				MaxResults: i + 100,
				//Determines how to validate the JQL query and treat the validation results.
				ValidateQuery: "strict", //strict Returns a 400 response code if any errors are found, along with a list of all errors (and warnings).
				Fields: []string{
					FieldDeveloperMap,
					FieldEpicKey,
					FieldSprintInfo,
					"timetracking",
					"timespent",
					"timeoriginalestimate",
					"summary",
					"status",
					"issuetype",
					"subtasks",
					"assignee",
					"parent",
				},
			},
		)

		if err != nil {
			logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", resp)).Error("can't take from jira all not closed issues")
			return nil, err
		}

		if len(allIssues) == 0 {
			break
		}

		for _, issue := range allIssues {
			issues = append(issues, Issue{issue})
		}
	}
	return issues, nil
}

// AssigneeOpenIssues searches Issues in all sprints which opened now and returning list with issues in this sprints list
func (j *Jira) AssigneeOpenIssues() ([]Issue, error) {
	request := fmt.Sprintf(`assignee != %s AND Sprint IN openSprints() AND (status NOT IN ("%s")) AND issuetype IN subTaskIssueTypes()`, StatusEmptyAssignee, StatusClosed)
	issues, err := j.issues(request)
	if err != nil {
		return nil, fmt.Errorf("can't take open jira issues type in subtasks of open sprints: %s", err)
	}
	return issues, nil
}

// IssuesWithClosedSubtasks retrieves issues with closed subtasks
func (j *Jira) IssuesWithClosedSubtasks() ([]Issue, error) {
	request := fmt.Sprintf(`status NOT IN ("%s") AND type in (story, bug) AND Sprint in openSprints()`, StatusClosed)
	openIssues, err := j.issues(request)
	if err != nil {
		return nil, err
	}
	var issuesWithSubtasks []Issue
	for _, issue := range openIssues {
		if len(issue.Fields.Subtasks) != 0 {
			issuesWithSubtasks = append(issuesWithSubtasks, issue)
		}
	}

	var issuesWithClosedSubtasks []Issue
	for _, issue := range issuesWithSubtasks {
		func() {
			for _, subtask := range issue.Fields.Subtasks {
				if subtask.Fields.Status.Name != StatusClosed {
					return
				}
			}
			issuesWithClosedSubtasks = append(issuesWithClosedSubtasks, issue)
		}()
	}
	return issuesWithClosedSubtasks, nil
}

// IssuesAfterSecondReview retrieves issues that have 2 or more reviews
func (j *Jira) IssuesAfterSecondReview() ([]Issue, error) {
	request := fmt.Sprintf(`status NOT IN ("%s") AND (status was "%s" OR status was "%s")`, StatusClosed, StatusTlReview, StatusPeerReview)
	issues, err := j.issues(request)
	if err != nil {
		return nil, err
	}
	var issuesAfterReview []Issue
	for _, i := range issues {
		issue, resp, err := j.Issue.Get(i.ID, &jira.GetQueryOptions{
			Expand:        i.Expand,
			UpdateHistory: true,
		})
		if err != nil {
			logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", resp)).Error("can't take from jira this jira issue")
			return nil, err
		}
		if len(issue.Changelog.Histories) == 0 {
			continue
		}

		countPeer := 0
		countTl := 0
		for _, histories := range issue.Changelog.Histories {
			for _, item := range histories.Items {
				if item.ToString == StatusPeerReview {
					countPeer++
				}
				if item.ToString == StatusTlReview {
					countTl++
				}
			}
		}
		if countPeer > 1 || countTl > 1 {
			issuesAfterReview = append(issuesAfterReview, i)
		}
	}
	return issuesAfterReview, nil
}

// IssuesClosedFromOpenSprint retrieves issues with closed status (bugs and stories)
func (j *Jira) IssuesClosedFromOpenSprint(project string) ([]Issue, error) {
	request := fmt.Sprintf(`status IN ("%s") AND project = %s AND type in (story, bug) AND sprint in openSprints() ORDER BY cf[10008] ASC, cf[10026] ASC`,
		StatusClosed, project)
	issues, err := j.issues(request)
	if err != nil {
		return nil, err
	}
	var issuesWithClosedStatus []Issue
	for _, issue := range issues {
		issuesWithClosedStatus = append(issuesWithClosedStatus, issue)
	}
	return issuesWithClosedStatus, nil
}

// IssuesClosedSubtasksFromOpenSprint retrieves issues with closed subtasks (bugs and stories)
func (j *Jira) IssuesClosedSubtasksFromOpenSprint(project string) ([]Issue, error) {
	request := fmt.Sprintf(`project = %s AND type in (story, bug) AND sprint in openSprints() ORDER BY cf[10008] ASC, cf[10026] ASC`, project)
	issues, err := j.issues(request)
	if err != nil {
		return nil, err
	}
	var issuesWithClosedSubtasks []Issue
Loop:
	for _, issue := range issues {
		for _, subtask := range issue.Fields.Subtasks {
			if subtask.Fields.Status.Name != StatusClosed {
				continue Loop
			}
		}
		issuesWithClosedSubtasks = append(issuesWithClosedSubtasks, issue)
	}
	return issuesWithClosedSubtasks, nil
}

// IssuesForNextSprint retrieves issues that stands for next sprint (bugs and stories)
func (j *Jira) IssuesForNextSprint(project string) ([]Issue, error) {
	request := fmt.Sprintf(`project = %s AND type in (story, bug) AND sprint in openSprints() ORDER BY cf[10008] ASC, cf[10026] ASC`, project)
	issues, err := j.issues(request)
	if err != nil {
		return nil, err
	}

	var issuesForNextSprint []Issue
Loop:
	for _, issue := range issues {
		for _, subtask := range issue.Fields.Subtasks {
			if subtask.Fields.Status.Name != StatusClosed {
				issuesForNextSprint = append(issuesForNextSprint, issue)
				continue Loop
			}
		}
	}
	return issues, nil
}

// IssuesFromFutureSprint retrieves issues from future sprint (bugs and stories)
func (j *Jira) IssuesFromFutureSprint(project string) ([]Issue, error) {
	request := fmt.Sprintf(`project = %s AND type in (story, bug) AND sprint in futureSprints() ORDER BY cf[10008] ASC, cf[10026] ASC`, project)
	issues, err := j.issues(request)
	if err != nil {
		return nil, err
	}
	return issues, nil
}

// EpicName retrieves issue summary
func (j *Jira) EpicName(issueKey string) (string, error) {
	options := jira.GetQueryOptions{}
	epicIssue, resp, err := j.Issue.Get(issueKey, &options)
	if err != nil {
		logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", resp)).Error("can't take from jira this jira issue")
		return "", err
	}

	return fmt.Sprint(epicIssue.Fields.Unknowns[FieldEpicName]), nil
}

// IssuesOfOpenSprints searches Issues in all sprints which opened now and returning list with issues in this sprints list
func (j *Jira) IssuesOfOpenSprints() ([]Issue, error) {
	request := fmt.Sprintf(`assignee != %s AND type not in (story, bug) AND Sprint IN openSprints()`, StatusEmptyAssignee)
	issues, err := j.issues(request)
	if err != nil {
		return nil, fmt.Errorf("can't take jira issues with type not in (story, bug) of open sprints: %s", err)
	}
	return issues, nil
}

// IssueSetPMReviewStatus set PM transition for issue
func (j *Jira) IssueSetStatusCloseLastTask(issueKey string) error {
	transitions, resp, err := j.Issue.GetTransitions(issueKey)
	if err != nil {
		logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", resp)).Error("can't take from jira transisions list of issue")
		return err
	}
	for _, transition := range transitions {
		if transition.Name == StatusCloseLastTask {
			resp, err := j.Issue.DoTransition(issueKey, transition.ID)
			if err != nil {
				logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", resp)).Error("can't do transition from transisions list of issue")
				return err
			}
			break
		}
	}
	return nil
}

// ClarificationIssuesOfOpenSprints searches Issues in open sprtints with clarification status
func (j *Jira) ClarificationIssuesOfOpenSprints() ([]Issue, error) {
	request := fmt.Sprintf(`assignee != %s AND status IN ("%s")`, StatusEmptyAssignee, StatusInClarification)
	issues, err := j.issues(request)
	if err != nil {
		return nil, fmt.Errorf("can't take jira issues with type not in (story, bug) of open sprints: %s", err)
	}
	return issues, nil
}
