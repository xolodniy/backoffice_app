package jira

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

// Changelog struct don't let go-jira dependency on Controller level
type Changelog struct {
	ID    string                `json:"id"`
	Items []jira.ChangelogItems `json:"items"`
}

// New creates new jira
func New(config *config.Jira) Jira {
	auth := jira.BasicAuthTransport{Username: config.Auth.Username, Password: config.Auth.Token}
	jiraClient, err := jira.NewClient(auth.Client(), config.APIUrl)
	if err != nil {
		panic(err)
	}
	return Jira{
		jiraClient,
	}
}

// Status variables for jql requests
var (
	StatusStarted                      = "Started"
	StatusClosed                       = "Closed"
	StatusOpen                         = "Open"
	StatusTlReview                     = "In TL review"
	StatusPeerReview                   = "In peer review"
	StatusDesignReview                 = "in Design review"
	StatusCTOReview                    = "In CTO review"
	StatusFEReview                     = "In FE review"
	StatusCloseLastTask                = "Close last task"
	StatusReadyForDemo                 = "Ready for demo"
	StatusEmptyAssignee                = "empty"
	StatusInClarification              = "In clarification"
	StatusInArtDirectorReview          = "In Art-director review"
	FieldEpicName                      = "customfield_10005"
	FieldEpicKey                       = "customfield_10008"
	FieldSprintInfo                    = "customfield_10010"
	FieldDeveloperMap                  = "customfield_10026"
	TypeBESubTask                      = "BE Sub-Task"
	TypeBETask                         = "BE Task"
	TypeFESubTask                      = "FE Sub-Task"
	TypeFETask                         = "FE Task"
	TypeStory                          = "Story"
	TypeBug                            = "Bug"
	TransitionCreatingDevSubtasks      = "Creating Dev Subtasks"
	TransitionCompleteSubtasksCreation = "Complete sub-tasks creation"
	TransitionStart                    = "Start"
	TransitionDone                     = "Done"
	TransitionApprove                  = "Approve"
	TagDeveloperName                   = "displayName"
	TagDeveloperID                     = "accountId"
	NoDeveloper                        = "No developer"
	ChangelogFieldFixVersion           = "Fix Version"
	ChangelogFieldPrioriy              = "priority"
	ChangelogFieldDueDate              = "duedate"
)

func (i Issue) String() string {
	message := fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s - %[2]s>: _%[3]s_\n",
		i.Key, i.Fields.Summary, i.Fields.Status.Name)
	return message
}

func (i Issue) Link() string {
	message := fmt.Sprintf("<https://theflow.atlassian.net/browse/%[1]s|%[1]s>", i.Key)
	return message
}

// DeveloperMap retrieves information about developer variable by key
func (i Issue) DeveloperMap(key string) string {
	var developerKeyInfo string
	// Convert to marshal map to find developer emailAddress of issue developer field
	developerMap, err := i.Fields.Unknowns.MarshalMap(FieldDeveloperMap)
	if err != nil {
		//can't make customfield_10026 map marshaling because field developer is empty
		return ""
	}
	if developerMap != nil {
		displayKeyInfo, ok := developerMap[key].(string)
		if !ok {
			logrus.WithField(key, fmt.Sprintf("%+v", developerMap[key])).
				Error("can't assert to string map emailAddress field")
		}
		developerKeyInfo = displayKeyInfo
	}
	return developerKeyInfo
}

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
					"worklog",
					"priority",
					"fixVersions",
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
func (j *Jira) IssuesAfterSecondReview(typeNames []string) ([]Issue, error) {
	request := fmt.Sprintf(`status NOT IN ("%s") AND (status was "%s" OR status was "%s" OR status was "%s" OR status was "%s" OR status was "%s")`,
		StatusClosed, StatusTlReview, StatusPeerReview, StatusDesignReview, StatusCTOReview, StatusFEReview)
	if len(typeNames) != 0 {
		// format of jql statuses `("FE Task")` or `("FE Sub-Task","FE Task")`
		request += ` AND type IN ("` + strings.Join(typeNames, `","`) + `")`
	}
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

		var (
			countPeer = 0
			countTl   = 0
		)
		for _, histories := range issue.Changelog.Histories {
			for _, item := range histories.Items {
				switch item.ToString {
				case StatusPeerReview:
					countPeer++
				case StatusTlReview:
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
	request := fmt.Sprintf(`status NOT IN ("%s") AND project = %s AND type in (story, bug) AND sprint in openSprints() ORDER BY cf[10008] ASC, cf[10026] ASC`,
		StatusClosed, project)
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
	request := fmt.Sprintf(`status NOT IN ("%s") AND project = %s AND type in (story, bug) AND sprint in openSprints() ORDER BY cf[10008] ASC, cf[10026] ASC`,
		StatusClosed, project)
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
	return issuesForNextSprint, nil
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

// IssuesStoryBugOfOpenSprints searches Issues in all sprints which opened now and returning list with issues in this sprints list (bugs and stories)
func (j *Jira) IssuesStoryBugOfOpenSprints(project string) ([]Issue, error) {
	request := fmt.Sprintf(`project = %s AND type in (story, bug) AND Sprint IN openSprints() ORDER BY cf[10008] ASC, cf[10026] ASC`, project)
	issues, err := j.issues(request)
	if err != nil {
		return nil, fmt.Errorf("can't take jira issues with type in (story, bug) of open sprints: %s", err)
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

// OpenIssuesOfOpenSprints searches Issues in all sprints which opened now and returning list with issues in this sprints list
func (j *Jira) OpenIssuesOfOpenSprints() ([]Issue, error) {
	request := fmt.Sprintf(`type not in (story, bug) AND status NOT IN ("%s") AND Sprint IN openSprints()`, StatusClosed)
	issues, err := j.issues(request)
	if err != nil {
		return nil, fmt.Errorf("can't take jira issues with type not in (story, bug) of open sprints: %s", err)
	}
	return issues, nil
}

// IssueSetStatusTransition set status close transition for issue
func (j *Jira) IssueSetStatusTransition(issueKey, transitionName string) error {
	transitions, resp, err := j.Issue.GetTransitions(issueKey)
	if err != nil {
		logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", resp)).Error("can't take from jira transisions list of issue")
		return err
	}
	for _, transition := range transitions {
		if transition.Name == transitionName {
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

// IssuesOnReview searches all issues with review statuses and retrieves it with changelog history
func (j *Jira) IssuesOnReview() ([]Issue, error) {
	request := fmt.Sprintf(`assignee != %s AND status IN ("%s","%s","%s","%s","%s")`,
		StatusEmptyAssignee, StatusPeerReview, StatusTlReview, StatusDesignReview, StatusCTOReview, StatusFEReview)
	issues, err := j.issues(request)
	if err != nil {
		return nil, fmt.Errorf("can't take jira not closed issues: %s", err)
	}
	var issuesOnReview []Issue
	for _, issue := range issues {
		issueWithChangelog, err := j.getIssueChangelog(issue)
		if err != nil {
			return nil, err
		}
		issuesOnReview = append(issuesOnReview, issueWithChangelog)
	}
	return issuesOnReview, nil
}

// RejectedIssueTLReviewCount retrieves count of TL review rejections if tl review was rejected at last
func (j *Jira) RejectedIssueTLReviewCount(issue Issue) (int, error) {
	issueWithHistory, err := j.getIssueChangelog(issue)
	if err != nil {
		return 0, err
	}
	var reviewCount int
	if issueWithHistory.Changelog == nil || len(issueWithHistory.Changelog.Histories) == 0 {
		return 0, nil
	}
	tlReviewRejected := false
	for _, item := range issueWithHistory.Changelog.Histories[len(issueWithHistory.Changelog.Histories)-1].Items {
		if item.FromString == StatusTlReview && item.ToString == StatusStarted {
			tlReviewRejected = true
		}
	}
	if tlReviewRejected == false {
		return 0, nil
	}
	for _, histories := range issueWithHistory.Changelog.Histories {
		for _, item := range histories.Items {
			if item.FromString == StatusTlReview {
				reviewCount++
			}
		}
	}
	return reviewCount, nil
}

// getIssueChangelog retrieves issue with change log history
func (j *Jira) getIssueChangelog(issue Issue) (Issue, error) {
	changeLog := &struct {
		MaxResults int                     `json:"maxResults"`
		StartAt    int                     `json:"startAt"`
		Total      int                     `json:"total"`
		IsLast     bool                    `json:"isLast"`
		Values     []jira.ChangelogHistory `json:"values"`
	}{}
	index := 0
	issue.Changelog = &jira.Changelog{}
	for {
		url := fmt.Sprintf("/rest/api/2/issue/%s/changelog?maxResults=100&startAt=%v", issue.Key, index)
		req, err := j.NewRequest("GET", url, nil)
		if err != nil {
			return Issue{}, fmt.Errorf("can't create request of changelog enpoint: %s", err)
		}
		_, err = j.Do(req, changeLog)
		if err != nil {
			return Issue{}, fmt.Errorf("can't take jira changelog of issue: %s", err)
		}
		for _, history := range changeLog.Values {
			issue.Changelog.Histories = append(issue.Changelog.Histories, history)
		}
		if changeLog.IsLast {
			break
		}
		index += 100
	}
	return issue, nil
}

// IssuesClosedInInterim retrieves isses closed in after dateStart and before dateEnd with not emmpty developer
func (j *Jira) IssuesClosedInInterim(dateStart, dateEnd time.Time) ([]Issue, error) {
	// this request retrieves closed and canceled issues
	request := fmt.Sprintf(`type not in (story, bug) and status changed to %s after %s before %s`,
		StatusClosed, dateStart.Format("2006-01-02"), dateEnd.Format("2006-01-02"))
	issues, err := j.issues(request)
	if err != nil {
		return nil, fmt.Errorf("can't take jira issues closed after %s before %s with error: %s",
			dateStart.Format("2006-01-02"), dateEnd.Format("2006-01-02"), err)
	}
	return issues, nil
}

// EpicsWithClosedIssues retrieves epics with closed issues
func (j *Jira) EpicsWithClosedIssues() ([]Issue, error) {
	request := fmt.Sprintf(`type in (Epic) AND status NOT IN ("%s")`, StatusClosed)
	openEpics, err := j.issues(request)
	if err != nil {
		return nil, err
	}
	var epicsWithClosedIssues []Issue
	for _, epic := range openEpics {
		if j.EpicIssuesClosed(epic.Key) {
			epicsWithClosedIssues = append(epicsWithClosedIssues, epic)
		}
	}
	return epicsWithClosedIssues, nil
}

// IssueType retrieve issue type name by issue id
func (j *Jira) IssueType(issueID string) (string, error) {
	issue, resp, err := j.Issue.Get(issueID, &jira.GetQueryOptions{})
	if err != nil {
		logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", resp)).Errorf("can't take from jira issue '%s' info", issueID)
		return "", err
	}
	return issue.Fields.Type.Name, nil
}

// IssueSubtasksClosed retrieves true if all subtasks of issue closed
func (j *Jira) IssueSubtasksClosed(issueID string) bool {
	issue, resp, err := j.Issue.Get(issueID, &jira.GetQueryOptions{})
	if err != nil {
		logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", resp)).Error("can't take from jira issue info")
		return false
	}
	if len(issue.Fields.Subtasks) == 0 {
		return false
	}
	for _, subtask := range issue.Fields.Subtasks {
		if subtask.Fields.Status.Name != StatusClosed {
			return false
		}
	}
	return true
}

// EpicIssuesClosed retrieves true if all issues of epic closed
func (j *Jira) EpicIssuesClosed(epicKey string) bool {
	epicIssues, err := j.issues(fmt.Sprintf(`cf[10008] = "%s"`, epicKey))
	if err != nil {
		logrus.WithError(err).Error("can't take from jira epic issues")
		return false
	}
	if len(epicIssues) == 0 {
		return false
	}
	for _, issue := range epicIssues {
		if issue.Fields.Status.Name != StatusClosed {
			return false
		}
	}
	return true
}

// UpdateIssueFixVersion update issue fix version by issue key
func (j *Jira) UpdateIssueFixVersion(issueKey, fromFixVersion, toFixVersion string) error {
	var byt []byte
	switch {
	case toFixVersion == "":
		byt = []byte(fmt.Sprintf(`{"update":{"fixVersions":[{"remove": {"name":"%s"} }]}}`, fromFixVersion))
	case fromFixVersion == "":
		byt = []byte(fmt.Sprintf(`{"update":{"fixVersions":[{"add": {"name":"%s"} }]}}`, toFixVersion))
	}
	var dat map[string]interface{}
	if err := json.Unmarshal(byt, &dat); err != nil {
		logrus.WithError(err).Error("can't unmarshal byte data for update fix version")
		return err
	}
	res, err := j.Issue.UpdateIssue(issueKey, dat)
	if err != nil {
		logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", res)).Error("can't update issue fix version")
		return err
	}
	return nil
}

// SetIssuePriority set issue priority by issue key
func (j *Jira) SetIssuePriority(issueKey, priority string) error {
	byt := []byte(fmt.Sprintf(`{"update":{"priority":[{"set":{"name" : "%s"}}]}}`, priority))
	var dat map[string]interface{}
	if err := json.Unmarshal(byt, &dat); err != nil {
		logrus.WithError(err).Error("can't unmarshal byte data for set priority")
		return err
	}
	res, err := j.Issue.UpdateIssue(issueKey, dat)
	if err != nil {
		logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", res)).Error("can't set issue priority")
		return err
	}
	return nil
}

// SetIssueDueDate set issue due date by issue key
func (j *Jira) SetIssueDueDate(issueKey, dueDate string) error {
	byt := []byte(fmt.Sprintf(`{"fields":{"duedate":"%s"}}`, dueDate))
	var dat map[string]interface{}
	if err := json.Unmarshal(byt, &dat); err != nil {
		logrus.WithError(err).Error("can't unmarshal byte data for set priority")
		return err
	}
	res, err := j.Issue.UpdateIssue(issueKey, dat)
	if err != nil {
		logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", res)).Error("can't set issue due date")
		return err
	}
	return nil
}

// OpenedIssuesWithDeveloper searches Issues in all sprints which opened or started with developer sorted by priority
func (j *Jira) OpenedIssuesWithLastWorklogActivity() ([]Issue, error) {
	request := fmt.Sprintf(`status in (%s, %s) AND Developer is not EMPTY ORDER BY priority DESC`, StatusOpen, StatusStarted)
	issues, err := j.issues(request)
	if err != nil {
		return nil, fmt.Errorf("can't take open jira issues type in subtasks of open sprints: %s", err)
	}
	for _, issue := range issues {
		lastWorklog, err := j.getIssueLastWorkLogActivity(issue.Key, issue.Fields.Worklog.Total)
		if err != nil {
			return issues, err
		}
		issue.Fields.Worklog = &lastWorklog
	}
	return issues, nil
}

// getIssueLastWorkLogActivity retrieves last worklog activity
func (j *Jira) getIssueLastWorkLogActivity(issueKey string, totalCount int) (jira.Worklog, error) {
	workLog := &jira.Worklog{}
	url := fmt.Sprintf("/rest/api/2/issue/%s/worklog?startAt=%v", issueKey, totalCount-1)
	req, err := j.NewRequest("GET", url, nil)
	if err != nil {
		return *workLog, fmt.Errorf("can't create request of worklog enpoint: %s", err)
	}
	_, err = j.Do(req, workLog)
	if err != nil {
		return *workLog, fmt.Errorf("can't take jira worklog of issue: %s", err)
	}
	if len(workLog.Worklogs) == 0 {
		return *workLog, nil
	}
	return *workLog, nil
}
