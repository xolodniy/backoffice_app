package bitbucket

import (
	"backoffice_app/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

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

// execute initialize and executes request. if pages of answer > 1 do autopaginate and return all slices
func (b *Bitbucket) do(urlStr, method string, jsonBody []byte) ([]byte, error) {
	req, err := http.NewRequest(method, urlStr, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(b.Auth.user, b.Auth.password)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// RepositoriesList returns list of all repositories
func (b *Bitbucket) RepositoriesList() ([]repository, error) {
	type repositories struct {
		Next   string       `json:"next"`
		Values []repository `json:"values"`
	}
	var repos = repositories{Next: b.Url + "/repositories/" + b.Owner}
	for {
		res, err := b.do(repos.Next, "GET", nil)
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
		res, err := b.do(pr.Next, "GET", nil)
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
		res, err := b.do(prCommits.Next, "GET", nil)
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
		res, err := b.do(diff.Next, "GET", nil)
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

// SrcFile returns files diff of commits by repository slug and commit hash
func (b *Bitbucket) SrcFile(repoSlug, spec, path string) (string, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/src/" + spec + "/" + path
	res, err := b.do(urlStr, "GET", nil)
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
			allCommits = append(allCommits, commit)
		}
	}
	return allCommits, nil
}

// repoSlugByIsueKey retrieves repo slug by issueKey
func (b *Bitbucket) repoSlugByProjectKey(projectKey string) (string, error) {
	type repo struct {
		Type   string       `json:"type"`
		Values []repository `json:"values"`
	}
	urlStr := b.Url + "/repositories/" + b.Owner + "?q=project.key=\"" + projectKey + "\""
	res, err := b.do(urlStr, "GET", nil)
	if err != nil {
		return "", err
	}
	var repositoryInfo repo
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
	res, err := b.do(urlStr, "GET", nil)
	if err != nil {
		return "", err
	}
	var branchInfo BranchInfo
	err = json.Unmarshal(res, &branchInfo)
	if err != nil {
		return "", err
	}
	if branchInfo.Type == "error" {
		return "", fmt.Errorf("Can't take branch hash with error message: %s ", branchInfo.Error.Message)
	}
	return branchInfo.Target.Hash, nil
}

// CreateBranch creates branch in repository
func (b *Bitbucket) createBranch(repoSlug, branchName, targetHash string) error {
	request := BranchInfo{Name: branchName, Target: struct {
		Hash string `json:"hash"`
	}{targetHash}}
	jsonReport, err := json.Marshal(request)
	if err != nil {
		logrus.WithError(err).Errorf("Can't convert last activity report to json. Report is:\n%s", request)
		return err
	}

	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/refs/branches"
	res, err := b.do(urlStr, "POST", jsonReport)
	if err != nil {
		return err
	}
	var CheckResponse = struct {
		Type  string `json:"type"`
		Error Error  `json:"error"`
	}{}
	err = json.Unmarshal(res, &CheckResponse)
	if err != nil {
		return err
	}
	if CheckResponse.Type == "error" {
		if CheckResponse.Error.Data.Key != "BRANCH_ALREADY_EXISTS" {
			return fmt.Errorf("Can't create branch with error message: %s ", CheckResponse.Error.Message)
		}
	}
	return nil
}

// FindTargetCommitAndCreateBranch get repo slug by project key, get target hash of parent branch, create branch
func (b *Bitbucket) FindTargetCommitAndCreateBranch(issueKey, branchName, branchParentName string) error {
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
	err = b.createBranch(repoSlug, branchName, targetHash)
	if err != nil {
		return err
	}
	return nil
}

// createPullRequest creates pull request in repository
func (b *Bitbucket) createPullRequest(repoSlug, branchName, branchParentName string) error {
	request := PullRequestCreateInfo{Title: "", Source: struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
	}{struct {
		Name string `json:"name"`
	}{branchName}}, Destination: struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
	}{struct {
		Name string `json:"name"`
	}{branchParentName}}}

	jsonReport, err := json.Marshal(request)
	if err != nil {
		logrus.WithError(err).Errorf("Can't convert last activity report to json. Report is:\n%s", request)
		return err
	}

	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests"
	res, err := b.do(urlStr, "POST", jsonReport)
	if err != nil {
		return err
	}
	var CheckResponse = struct {
		Type  string `json:"type"`
		Error Error  `json:"error"`
	}{}
	err = json.Unmarshal(res, &CheckResponse)
	if err != nil {
		return err
	}
	if CheckResponse.Type == "error" {
		if CheckResponse.Error.Message != "There are no changes to be pulled" {
			return fmt.Errorf("Can't create pull request of brarnch with error message: %s ", CheckResponse.Error.Message)
		}
	}
	return nil
}

// CheckPullRequestExistAndCreate check for existing pullrequest, if don't create new
func (b *Bitbucket) CheckPullRequestExistAndCreate(repoSlug, branchName, branchParentName string) error {
	pullRequestsList, err := b.PullRequestsList(repoSlug)
	if err != nil {
		return err
	}
	for _, pullRequest := range pullRequestsList {
		if pullRequest.Source.Branch.Name == branchName {
			return nil
		}
	}
	err = b.createPullRequest(repoSlug, branchName, branchParentName)
	if err != nil {
		return err
	}
	return nil
}
