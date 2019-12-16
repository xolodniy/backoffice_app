package bitbucket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"backoffice_app/common"
	"backoffice_app/config"

	"github.com/sirupsen/logrus"
)

// Bitbucket main struct of jira client
type Bitbucket struct {
	Auth  *auth
	Owner string
	Url   string
}

type auth struct {
	appID, secret  string
	user, password string
}

// New creates new Bitbucket
func New(config *config.Bitbucket) Bitbucket {
	return Bitbucket{
		Auth:  &auth{user: config.Auth.Username, password: config.Auth.Password},
		Owner: config.Owner,
		Url:   config.APIUrl,
	}
}

// do executes http request
func (b *Bitbucket) do(request *http.Request) ([]byte, error) {
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var CheckResponse = struct {
		Type  string `json:"type"`
		Error struct {
			Message string `json:"message"`
			Data    struct {
				Key string `json:"key"`
			} `json:"data"`
		} `json:"error"`
	}{}
	err = json.Unmarshal(body, &CheckResponse)
	// if can't unmarshal it means, that struct of answer hasn't error struct
	if err != nil {
		return body, nil
	}
	if CheckResponse.Type == "error" {
		if CheckResponse.Error.Message != "There are no changes to be pulled" && CheckResponse.Error.Data.Key != "BRANCH_ALREADY_EXISTS" {
			return nil, fmt.Errorf("Request was done with error: %s ", CheckResponse.Error.Message)
		}
	}
	return body, nil
}

// get prepare http request by get method
func (b *Bitbucket) get(urlStr string) ([]byte, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(b.Auth.user, b.Auth.password)
	respBody, err := b.do(req)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

// post prepare post request by post method
func (b *Bitbucket) post(urlStr string, jsonBody []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", urlStr, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(b.Auth.user, b.Auth.password)
	respBody, err := b.do(req)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

// get prepare http request by get method
func (b *Bitbucket) delete(urlStr string) ([]byte, error) {
	req, err := http.NewRequest("DELETE", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(b.Auth.user, b.Auth.password)
	respBody, err := b.do(req)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

// RepositoriesList returns list of all repositories
func (b *Bitbucket) RepositoriesList() ([]repository, error) {
	type repositories struct {
		Next   string       `json:"next"`
		Values []repository `json:"values"`
	}
	var repos = repositories{Next: b.Url + "/repositories/" + b.Owner}
	for {
		res, err := b.get(repos.Next)
		if err != nil {
			return []repository{}, err
		}
		var nextRepositories repositories
		err = json.Unmarshal(res, &nextRepositories)
		if err != nil {
			return []repository{}, err
		}
		for _, repository := range nextRepositories.Values {
			repos.Values = append(repos.Values, repository)
		}
		if nextRepositories.Next == "" {
			break
		}
		repos.Next = nextRepositories.Next
	}
	return repos.Values, nil
}

// PullRequestsList returns list of pull requests in repository by repository slug
func (b *Bitbucket) PullRequestsList(repoSlug string) ([]pullRequest, error) {
	type pullRequests struct {
		Next   string        `json:"next"`
		Values []pullRequest `json:"values"`
	}
	var pr = pullRequests{Next: b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests?state=OPEN"}
	for {
		res, err := b.get(pr.Next)
		if err != nil {
			return []pullRequest{}, err
		}
		var nextPullRequests pullRequests
		err = json.Unmarshal(res, &nextPullRequests)
		if err != nil {
			return []pullRequest{}, err
		}
		for _, pullRequest := range nextPullRequests.Values {
			pr.Values = append(pr.Values, pullRequest)
		}
		if nextPullRequests.Next == "" {
			break
		}
		pr.Next = nextPullRequests.Next
	}
	return pr.Values, nil
}

// PullRequestCommits returns commits in pull request by pull request id and repository slug
func (b *Bitbucket) PullRequestCommits(repoSlug, prID string) ([]Commit, error) {
	type commits struct {
		Next   string   `json:"next"`
		Values []Commit `json:"values"`
	}
	var prCommits = commits{Next: b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests/" + prID + "/commits"}
	for {
		res, err := b.get(prCommits.Next)
		if err != nil {
			return []Commit{}, err
		}
		var nextCommits commits
		err = json.Unmarshal(res, &nextCommits)
		if err != nil {
			return []Commit{}, err
		}
		for _, commit := range nextCommits.Values {
			prCommits.Values = append(prCommits.Values, commit)
		}
		if nextCommits.Next == "" {
			break
		}
		prCommits.Next = nextCommits.Next
	}
	return prCommits.Values, nil
}

// CommitsDiffStats returns files diff of commits by repository slug and commit hash
func (b *Bitbucket) CommitsDiffStats(repoSlug, spec string) ([]diffStat, error) {
	type diffStats struct {
		Next   string     `json:"next"`
		Values []diffStat `json:"values"`
	}
	var diff = diffStats{Next: b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/diffstat/" + spec}
	for {
		res, err := b.get(diff.Next)
		if err != nil {
			return []diffStat{}, err
		}
		var nextDiffStats diffStats
		err = json.Unmarshal(res, &nextDiffStats)
		if err != nil {
			return []diffStat{}, err
		}
		for _, pullRequest := range nextDiffStats.Values {
			diff.Values = append(diff.Values, pullRequest)
		}
		if nextDiffStats.Next == "" {
			break
		}
		diff.Next = nextDiffStats.Next
	}
	return diff.Values, nil
}

// PullRequestDiff returns pull request diff of repository
func (b *Bitbucket) PullRequestDiff(repoSlug string, pullRequestID int64) (string, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests/" + strconv.FormatInt(pullRequestID, 10) + "/diff"
	res, err := b.get(urlStr)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

// SrcFile returns files diff of commits by repository slug and commit hash
func (b *Bitbucket) SrcFile(repoSlug, spec, path string) (string, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/src/" + spec + "/" + path
	res, err := b.get(urlStr)
	if err != nil {
		return "", err
	}
	file := string(res)
	return file, nil
}

// CommitsOfOpenedPRs returns commits with migration diff
func (b *Bitbucket) CommitsOfOpenedPRs() ([]Commit, error) {
	repositories, err := b.RepositoriesList()
	if err != nil {
		return nil, err
	}

	var allPullRequests []pullRequest
	for _, repository := range repositories {
		pullRequests, err := b.PullRequestsList(repository.Name)
		if err != nil {
			return nil, err
		}
		for _, pullRequest := range pullRequests {
			allPullRequests = append(allPullRequests, pullRequest)
		}
	}

	var allCommits []Commit
	for _, pullRequest := range allPullRequests {
		commits, err := b.PullRequestCommits(pullRequest.Source.Repository.Name, strconv.FormatInt(pullRequest.ID, 10))
		if err != nil {
			return nil, err
		}
		for _, commit := range commits {
			// without merge commits
			if len(commit.Parents) > 1 {
				continue
			}
			allCommits = append(allCommits, commit)
		}
	}
	return allCommits, nil
}

// DiffFile returns diff of file in commits by repository slug and commit hash
func (b *Bitbucket) DiffFile(repoSlug, spec, path string) (string, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/diff/" + spec + "?path=" + path
	res, err := b.get(urlStr)
	if err != nil {
		return "", err
	}
	fileDiff := string(res)
	return fileDiff, nil
}

// repoSlugByIsueKey retrieves repo slug by issueKey
func (b *Bitbucket) repoSlugByProjectKey(projectKey string) (string, error) {
	var repositoryInfo struct {
		Type   string       `json:"type"`
		Values []repository `json:"values"`
	}
	urlStr := b.Url + "/repositories/" + b.Owner + "?q=project.key=\"" + projectKey + "\""
	res, err := b.get(urlStr)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(res, &repositoryInfo)
	if err != nil {
		return "", err
	}
	if len(repositoryInfo.Values) == 0 {
		return "", fmt.Errorf("There are no projects with \"%s\" project key ", projectKey)
	}
	return repositoryInfo.Values[0].Slug, nil
}

// branchTargetCommitHash retrieves hash of branch
func (b *Bitbucket) branchTargetCommitHash(repoSlug, branchName string) (string, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/refs/branches/" + branchName
	res, err := b.get(urlStr)
	if err != nil {
		return "", err
	}
	var branchInfo struct {
		Type   string `json:"type"`
		Name   string `json:"name"`
		Target struct {
			Hash string `json:"hash"`
		} `json:"target"`
	}
	err = json.Unmarshal(res, &branchInfo)
	if err != nil {
		return "", err
	}
	return branchInfo.Target.Hash, nil
}

// CreateBranch create branch in project repository by issueKey, branchName and branchParentName
func (b *Bitbucket) CreateBranch(issueKey, branchName, branchParentName string) error {
	issueKeySlice := strings.Split(issueKey, "-")
	if len(issueKeySlice) != 2 {
		return fmt.Errorf("can't take project key from issue key \"%s\", format must be KEY-1", issueKey)
	}
	repoSlug, err := b.repoSlugByProjectKey(issueKeySlice[0])
	if err != nil {
		return err
	}
	targetHash, err := b.branchTargetCommitHash(repoSlug, branchParentName)
	if err != nil {
		return err
	}

	requestBody := []byte(fmt.Sprintf(`{"name":"%s","target":{"hash":"%s"}}`, branchName, targetHash))
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/refs/branches"
	res, err := b.post(urlStr, requestBody)
	if err != nil {
		logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", res)).Error("can't create branch in bitbucket")
		return err
	}
	return nil
}

// CreatePullRequestIfNotExist find pull requests and create new if not exist
func (b *Bitbucket) CreatePullRequestIfNotExist(repoSlug, branchName, branchParentName string) error {
	pullRequestsList, err := b.PullRequestsList(repoSlug)
	if err != nil {
		return err
	}
	for _, pullRequest := range pullRequestsList {
		if pullRequest.Source.Branch.Name == branchName {
			return nil
		}
	}

	requestBody := []byte(fmt.Sprintf(`{"title": "%[1]s", "source":{"branch":{"name": "%[1]s"}}, "destination":{"branch":{"name": "%[2]s"}}}`,
		branchName, branchParentName))
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests"
	res, err := b.post(urlStr, requestBody)
	if err != nil {
		logrus.WithError(err).WithField("response", fmt.Sprintf("%+v", res)).Error("can't create pull request in bitbucket")
		return err
	}
	return nil
}

// pullRequestActivities returns list of pull requests  activities in repository by repository slug and pullRequestID
func (b *Bitbucket) pullRequestActivity(repoSlug, pullRequestID string) ([]pullRequestActivity, error) {
	type pullRequestActivities struct {
		Next   string                `json:"next"`
		Values []pullRequestActivity `json:"values"`
	}
	var pr = pullRequestActivities{Next: b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests/" + pullRequestID + "/activity"}
	for {
		res, err := b.get(pr.Next)
		if err != nil {
			return []pullRequestActivity{}, err
		}
		var nextPullRequests pullRequestActivities
		err = json.Unmarshal(res, &nextPullRequests)
		if err != nil {
			return []pullRequestActivity{}, err
		}
		for _, pullRequest := range nextPullRequests.Values {
			pr.Values = append(pr.Values, pullRequest)
		}
		if nextPullRequests.Next == "" {
			break
		}
		pr.Next = nextPullRequests.Next
	}
	return pr.Values, nil
}

// PullRequestsActivity returns pull requests with activity
func (b *Bitbucket) PullRequestsActivity() ([]pullRequest, error) {
	repositories, err := b.RepositoriesList()
	if err != nil {
		return nil, err
	}

	var allPullRequests []pullRequest
	for _, repository := range repositories {
		pullRequests, err := b.PullRequestsList(repository.Name)
		if err != nil {
			return nil, err
		}
		for _, pullRequest := range pullRequests {
			allPullRequests = append(allPullRequests, pullRequest)
		}
	}

	for i, pullRequest := range allPullRequests {
		activities, err := b.pullRequestActivity(pullRequest.Source.Repository.Name, strconv.FormatInt(pullRequest.ID, 10))
		if err != nil {
			return nil, err
		}
		for _, activity := range activities {
			allPullRequests[i].Activities = append(allPullRequests[i].Activities, activity)
		}
	}
	return allPullRequests, nil
}

// DeclinePullRequest declines pull request
func (b *Bitbucket) DeclinePullRequest(repoSlug string, pullRequestID int64) error {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests/" + strconv.FormatInt(pullRequestID, 10) + "/decline"
	res, err := b.post(urlStr, []byte{})
	if err != nil {
		return err
	}
	var checkResponse = struct {
		Type  string `json:"type"`
		Error struct {
			Message string `json:"message"`
			Data    struct {
				Key string `json:"key"`
			} `json:"data"`
		} `json:"error"`
	}{}
	err = json.Unmarshal(res, &checkResponse)
	if err != nil {
		return err
	}
	if checkResponse.Error.Message != "" {
		return fmt.Errorf(checkResponse.Error.Message)
	}
	return nil
}

func (b *Bitbucket) BranchesWithoutPullRequests() ([]branch, error) {
	repositories, err := b.RepositoriesList()
	if err != nil {
		return nil, err
	}

	var branchesWithoutPullRequests []branch
	for _, repository := range repositories {
		var branchesWithPullRequests []string
		pullRequests, err := b.PullRequestsList(repository.Name)
		if err != nil {
			return nil, err
		}
		for _, pullRequest := range pullRequests {
			branchesWithPullRequests = append(branchesWithPullRequests, pullRequest.Source.Branch.Name)
		}
		branches, err := b.BranchesList(repository.Name)
		if err != nil {
			return nil, err
		}
		for _, branch := range branches {
			//TODO add to config names of branches, that we can't take
			if !common.ValueIn(branch.Name, branchesWithPullRequests...) && branch.Name != "master" {
				branchesWithoutPullRequests = append(branchesWithoutPullRequests, branch)
			}
		}
	}
	return branchesWithoutPullRequests, nil
}

// BranchesList returns list of branches in repository by repository slug
func (b *Bitbucket) BranchesList(repoSlug string) ([]branch, error) {
	type paginatedBranches struct {
		Next   string   `json:"next"`
		Values []branch `json:"values"`
	}
	var pb = paginatedBranches{Next: b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/refs/branches?state=OPEN"}
	for {
		res, err := b.get(pb.Next)
		if err != nil {
			return []branch{}, err
		}
		var paginatedBranches paginatedBranches
		err = json.Unmarshal(res, &paginatedBranches)
		if err != nil {
			return []branch{}, err
		}
		for _, branch := range paginatedBranches.Values {
			pb.Values = append(pb.Values, branch)
		}
		if paginatedBranches.Next == "" {
			break
		}
		pb.Next = paginatedBranches.Next
	}
	return pb.Values, nil
}

// DeleteBranch deletes branch
func (b *Bitbucket) DeleteBranch(repoSlug, branchName string) error {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/refs/branches/" + branchName
	res, err := b.delete(urlStr)
	if err != nil {
		return err
	}
	var checkResponse = struct {
		Type  string `json:"type"`
		Error struct {
			Message string `json:"message"`
			Data    struct {
				Key string `json:"key"`
			} `json:"data"`
		} `json:"error"`
	}{}
	err = json.Unmarshal(res, &checkResponse)
	if err != nil {
		return err
	}
	if checkResponse.Error.Message != "" {
		return fmt.Errorf(checkResponse.Error.Message)
	}
	return nil
}
