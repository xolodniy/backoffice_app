package bitbucket

import (
	"backoffice_app/config"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// Bitbucket main struct of jira client
type Bitbucket struct {
	*Client
}

type Client struct {
	Auth       *auth
	Pagelen    uint64
	Owner      string
	Url        string
	HttpClient *http.Client
}
type auth struct {
	appID, secret  string
	user, password string
}

// New creates new Bitbucket
func New(config *config.Bitbucket) Bitbucket {
	return Bitbucket{
		&Client{
			Auth:       &auth{user: config.Auth.Username, password: config.Auth.Password},
			Owner:      config.Owner,
			Url:        config.APIUrl,
			HttpClient: new(http.Client),
		},
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
	resp, err := b.HttpClient.Do(req)
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
func (b *Bitbucket) RepositoriesList() (Repositories, error) {
	urlStr := b.Url + "/repositories/" + b.Owner
	res, err := b.do(urlStr)
	if err != nil {
		return Repositories{}, err
	}
	var repositories Repositories
	err = json.Unmarshal(res, &repositories)
	if err != nil {
		return Repositories{}, err
	}
	for repositories.Next != "" {
		res, err := b.do(repositories.Next)
		if err != nil {
			return Repositories{}, err
		}
		var nextRepositories Repositories
		err = json.Unmarshal(res, &nextRepositories)
		if err != nil {
			return Repositories{}, err
		}
		repositories.Next = nextRepositories.Next
		for _, repository := range nextRepositories.Values {
			repositories.Values = append(repositories.Values, repository)
		}
	}
	return repositories, nil
}

// PullRequestsList returns list of pull requests in repository by repository slug
func (b *Bitbucket) PullRequestsList(repoSlug string) (PullRequests, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests/"
	res, err := b.do(urlStr)
	if err != nil {
		return PullRequests{}, err
	}

	var pullRequests PullRequests
	err = json.Unmarshal(res, &pullRequests)
	if err != nil {
		return PullRequests{}, err
	}
	for pullRequests.Next != "" {
		res, err := b.do(pullRequests.Next)
		if err != nil {
			return PullRequests{}, err
		}
		var nextPullRequests PullRequests
		err = json.Unmarshal(res, &nextPullRequests)
		if err != nil {
			return PullRequests{}, err
		}
		pullRequests.Next = nextPullRequests.Next
		for _, pullRequest := range nextPullRequests.Values {
			pullRequests.Values = append(pullRequests.Values, pullRequest)
		}
	}
	return pullRequests, nil
}

// PullRequestCommits returns commits in pull request by pull request id and repository slug
func (b *Bitbucket) PullRequestCommits(repoSlug, prID string) (Commits, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests/" + prID + "/commits"
	res, err := b.do(urlStr)
	if err != nil {
		return Commits{}, err
	}
	var commits Commits
	err = json.Unmarshal(res, &commits)
	if err != nil {
		return Commits{}, err
	}
	for commits.Next != "" {
		res, err := b.do(commits.Next)
		if err != nil {
			return Commits{}, err
		}
		var nextCommits Commits
		err = json.Unmarshal(res, &nextCommits)
		if err != nil {
			return Commits{}, err
		}
		commits.Next = nextCommits.Next
		for _, commit := range nextCommits.Values {
			commits.Values = append(commits.Values, commit)
		}
	}
	return commits, nil
}

// CommitsDiff returns files diff of commits by repository slug and commit hash
func (b *Bitbucket) CommitsDiffStats(repoSlug, spec string) (DiffStats, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/diffstat/" + spec
	res, err := b.do(urlStr)
	if err != nil {
		return DiffStats{}, err
	}

	var diffStats DiffStats
	err = json.Unmarshal(res, &diffStats)
	if err != nil {
		return DiffStats{}, err
	}
	for diffStats.Next != "" {
		res, err := b.do(diffStats.Next)
		if err != nil {
			return DiffStats{}, err
		}
		var nextDiffStats DiffStats
		err = json.Unmarshal(res, &nextDiffStats)
		if err != nil {
			return DiffStats{}, err
		}
		diffStats.Next = nextDiffStats.Next
		for _, pullRequest := range nextDiffStats.Values {
			diffStats.Values = append(diffStats.Values, pullRequest)
		}
	}
	return diffStats, nil
}

// CommitsDiff returns files diff of commits by repository slug and commit hash
func (b *Bitbucket) SrcFile(repoSlug, spec, path string) (string, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/src/" + spec + "/" + path
	res, err := b.do(urlStr)
	if err != nil {
		return "", err
	}
	file := string(res[:])
	return file, nil
}
