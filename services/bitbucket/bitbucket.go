package bitbucket

import (
	"backoffice_app/config"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
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
func (b *Bitbucket) RepositoriesList() (repositories, error) {
	urlStr := b.Url + "/repositories/" + b.Owner
	res, err := b.do(urlStr)
	if err != nil {
		return repositories{}, err
	}
	var repos repositories
	err = json.Unmarshal(res, &repos)
	if err != nil {
		return repositories{}, err
	}
	for {
		if repos.Next == "" {
			break
		}
		res, err := b.do(repos.Next)
		if err != nil {
			return repositories{}, err
		}
		var nextRepositories repositories
		err = json.Unmarshal(res, &nextRepositories)
		if err != nil {
			return repositories{}, err
		}
		repos.Next = nextRepositories.Next
		for _, repository := range nextRepositories.Values {
			repos.Values = append(repos.Values, repository)
		}
	}
	return repos, nil
}

// PullRequestsList returns list of pull requests in repository by repository slug
func (b *Bitbucket) PullRequestsList(repoSlug string) (pullRequests, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests/"
	res, err := b.do(urlStr)
	if err != nil {
		return pullRequests{}, err
	}

	var pr pullRequests
	err = json.Unmarshal(res, &pr)
	if err != nil {
		return pullRequests{}, err
	}
	for pr.Next != "" {
		res, err := b.do(pr.Next)
		if err != nil {
			return pullRequests{}, err
		}
		var nextPullRequests pullRequests
		err = json.Unmarshal(res, &nextPullRequests)
		if err != nil {
			return pullRequests{}, err
		}
		pr.Next = nextPullRequests.Next
		for _, pullRequest := range nextPullRequests.Values {
			pr.Values = append(pr.Values, pullRequest)
		}
	}
	return pr, nil
}

// PullRequestCommits returns commits in pull request by pull request id and repository slug
func (b *Bitbucket) PullRequestCommits(repoSlug, prID string) (commits, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests/" + prID + "/commits"
	res, err := b.do(urlStr)
	if err != nil {
		return commits{}, err
	}
	var prCommits commits
	err = json.Unmarshal(res, &prCommits)
	if err != nil {
		return commits{}, err
	}
	for prCommits.Next != "" {
		res, err := b.do(prCommits.Next)
		if err != nil {
			return commits{}, err
		}
		var nextCommits commits
		err = json.Unmarshal(res, &nextCommits)
		if err != nil {
			return commits{}, err
		}
		prCommits.Next = nextCommits.Next
		for _, commit := range nextCommits.Values {
			prCommits.Values = append(prCommits.Values, commit)
		}
	}
	return prCommits, nil
}

// CommitsDiff returns files diff of commits by repository slug and commit hash
func (b *Bitbucket) CommitsDiffStats(repoSlug, spec string) (diffStats, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/diffstat/" + spec
	res, err := b.do(urlStr)
	if err != nil {
		return diffStats{}, err
	}

	var diff diffStats
	err = json.Unmarshal(res, &diff)
	if err != nil {
		return diffStats{}, err
	}
	for diff.Next != "" {
		res, err := b.do(diff.Next)
		if err != nil {
			return diffStats{}, err
		}
		var nextDiffStats diffStats
		err = json.Unmarshal(res, &nextDiffStats)
		if err != nil {
			return diffStats{}, err
		}
		diff.Next = nextDiffStats.Next
		for _, pullRequest := range nextDiffStats.Values {
			diff.Values = append(diff.Values, pullRequest)
		}
	}
	return diff, nil
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
func (b *Bitbucket) MigrationCommitsOfOpenedPRs() (map[string]HashCache, error) {
	repositories, err := b.RepositoriesList()
	if err != nil {
		return nil, err
	}

	var allPullRequests pullRequests
	for _, repository := range repositories.Values {
		pullRequests, err := b.PullRequestsList(repository.Name)
		if err != nil {
			return nil, err
		}
		for _, pullRequest := range pullRequests.Values {
			if pullRequest.State == "OPEN" {
				allPullRequests.Values = append(allPullRequests.Values, pullRequest)
			}
		}
	}

	newMapSqlCommits := make(map[string]HashCache)
	for _, pullRequest := range allPullRequests.Values {
		commits, err := b.PullRequestCommits(pullRequest.Source.Repository.Name, strconv.FormatInt(pullRequest.ID, 10))
		if err != nil {
			return nil, err
		}
		for _, commit := range commits.Values {
			diffStats, err := b.CommitsDiffStats(commit.Repository.Name, commit.Hash)
			if err != nil {
				return nil, err
			}
			for _, diffStat := range diffStats.Values {
				if strings.Contains(diffStat.New.Path, ".sql") {
					newMapSqlCommits[commit.Hash] = HashCache{Repository: commit.Repository.Name, Path: diffStat.New.Path, Message: commit.Message}
				}
			}
		}
	}
	return newMapSqlCommits, nil
}
