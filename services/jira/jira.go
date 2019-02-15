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
	StatusClosed        = "Closed"
	StatusTlReview      = "TL Review"
	StatusPeerReview    = "In peer review"
	StatusEmptyAssignee = "empty"
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
					"customfield_10026",
					"timetracking",
					"timespent",
					"timeoriginalestimate",
					"summary",
					"status",
					"issuetype",
					"subtasks",
					"assignee",
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
	request := fmt.Sprintf(`assignee != "%s" AND Sprint IN openSprints() AND (status NOT IN ("%s")) AND issuetype IN subTaskIssueTypes()`, StatusEmptyAssignee, StatusClosed)
	issues, err := j.issues(request)
	if err != nil {
		return nil, fmt.Errorf("can't create jira client: %s", err)
	}
	return issues, nil
}

// IssueTimeExceededNoTimeRange prepares string without employee time excess
func (j *Jira) IssueTimeExceededNoTimeRange(issue Issue, rowIndex int) string {
	if issue.Fields == nil {
		logrus.WithField("issue", fmt.Sprintf("%+v", issue)).Error("issue fields is empty")
		return ""
	}

	var listRow string

	if issue.Fields.TimeTracking.RemainingEstimateSeconds != 0 {
		return listRow
	}

	//TODO разобраться со вложенностями
	var developer = "No developer"
	developerMap, err := issue.Fields.Unknowns.MarshalMap("customfield_10026")
	if err != nil {
		logrus.WithError(err).WithField("developerMap", fmt.Sprintf("%+v", developerMap)).
			Error("can't make customfield_10026 map marshaling")
	} else if developerMap != nil {
		displayName, ok := developerMap["displayName"].(string)
		if !ok {
			logrus.WithField("displayName", fmt.Sprintf("%+v", developerMap["displayName"])).
				Error("can't assert to string map displayName field")
		} else {
			developer = displayName
		}
	}
	var worklogString string
	if issue.Fields.TimeTracking.TimeSpentSeconds > issue.Fields.TimeTracking.OriginalEstimateSeconds {
		worklogString = fmt.Sprintf(" time spent is %s instead %s", issue.Fields.TimeTracking.TimeSpent, issue.Fields.TimeTracking.OriginalEstimate)
	}

	listRow = fmt.Sprintf("%[1]d. %[2]s - <https://theflow.atlassian.net/browse/%[3]s|%[3]s - %[4]s>: _%[5]s_%[6]s\n",
		rowIndex, developer, issue.Key, issue.Fields.Summary, issue.Fields.Status.Name,
		worklogString)

	return listRow
}

// IssuesWithClosedSubtasks retrieves issues with closed subtasks
func (j *Jira) IssuesWithClosedSubtasks() ([]Issue, error) {
	request := fmt.Sprintf(`status NOT IN ("%s")  AND Sprint in openSprints()`, StatusClosed)
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
