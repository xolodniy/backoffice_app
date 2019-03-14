package bitbucket

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	"backoffice_app/config"
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
func (b *Bitbucket) do(urlStr string) ([]byte, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
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
		res, err := b.do(repos.Next)
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
		res, err := b.do(pr.Next)
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
		res, err := b.do(prCommits.Next)
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

// CommitsDiff returns files diff of commits by repository slug and commit hash
func (b *Bitbucket) CommitsDiffStats(repoSlug, spec string) ([]diffStat, error) {
	type diffStats struct {
		Next   string     `json:"next"`
		Values []diffStat `json:"values"`
	}
	var diff = diffStats{Next: b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/diffstat/" + spec}
	for {
		res, err := b.do(diff.Next)
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
	res, err := b.do(urlStr)
	if err != nil {
		return "", err
	}
	file := string(res)
	return file, nil
}

// MigrationCommitsOfOpenedPRs returns commits with migration diff
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
