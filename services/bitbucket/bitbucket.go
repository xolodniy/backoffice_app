package bitbucket

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"backoffice_app/config"

	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
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
	AllCache   map[string][]map[int64]string
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
			Pagelen:    10,
			Owner:      config.Owner,
			Url:        config.APIUrl,
			AllCache:   make(map[string][]map[int64]string),
			HttpClient: new(http.Client),
		},
	}
}

// execute initialize and executes request. if pages of answer > 1 do autopaginate and return all slices
func (b *Bitbucket) execute(method string, urlStr string, text string) (interface{}, error) {
	if strings.Contains(urlStr, "/repositories/") {
		if b.Pagelen != 10 {
			urlObj, err := url.Parse(urlStr)
			if err != nil {
				return nil, err
			}
			q := urlObj.Query()
			q.Set("pagelen", strconv.FormatUint(b.Pagelen, 10))
			urlObj.RawQuery = q.Encode()
			urlStr = urlObj.String()
		}
	}
	// if answer will not be json, returns respone string in interface()
	var isDiffPatchSrc bool
	if strings.Contains(urlStr, "/diff/") || strings.Contains(urlStr, "/patch/") || strings.Contains(urlStr, "/src/") {
		isDiffPatchSrc = true
	}

	body := strings.NewReader(text)

	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}
	if text != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	req.SetBasicAuth(b.Auth.user, b.Auth.password)
	result, err := b.doRequest(req, isDiffPatchSrc)
	if err != nil {
		return nil, err
	}

	//autopaginate.
	resultMap, isMap := result.(map[string]interface{})
	if isMap {
		nextIn := resultMap["next"]
		valuesIn := resultMap["values"]
		if nextIn != nil && valuesIn != nil {
			nextUrl := nextIn.(string)
			if nextUrl != "" {
				valuesSlice := valuesIn.([]interface{})
				if valuesSlice != nil {
					nextResult, err := b.execute(method, nextUrl, text)
					if err != nil {
						return nil, err
					}
					nextResultMap, isNextMap := nextResult.(map[string]interface{})
					if !isNextMap {
						return nil, fmt.Errorf("next page result is not map, it's %T", nextResult)
					}
					nextValuesIn := nextResultMap["values"]
					if nextValuesIn == nil {
						return nil, fmt.Errorf("next page result has no values")
					}
					nextValuesSlice, isSlice := nextValuesIn.([]interface{})
					if !isSlice {
						return nil, fmt.Errorf("next page result 'values' is not slice")
					}
					valuesSlice = append(valuesSlice, nextValuesSlice...)
					resultMap["values"] = valuesSlice
					delete(resultMap, "page")
					delete(resultMap, "pagelen")
					delete(resultMap, "size")
					result = resultMap
				}
			}
		}
	}

	return result, nil
}

// doRequest send request to server and return response
func (b *Bitbucket) doRequest(req *http.Request, isDiffPatchSrc bool) (interface{}, error) {

	resp, err := b.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if (resp.StatusCode != http.StatusOK) && (resp.StatusCode != http.StatusCreated) {
		return nil, fmt.Errorf(resp.Status)
	}

	if resp.Body == nil {
		return nil, fmt.Errorf("response body is nil")
	}

	resBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if isDiffPatchSrc {
		return string(resBodyBytes), nil
	}

	var result interface{}
	err = json.Unmarshal(resBodyBytes, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// RepositoriesList returns list of all repositories
func (b *Bitbucket) RepositoriesList() (Repositories, error) {
	urlStr := b.Url + "/repositories/" + b.Owner
	res, err := b.execute("GET", urlStr, "")
	if err != nil {
		return Repositories{}, err
	}
	var repositories Repositories
	err = mapstructure.Decode(res, &repositories)
	if err != nil {
		return Repositories{}, err
	}
	return repositories, nil
}

// PullRequestsList returns list of pull requests in repository by repository slug
func (b *Bitbucket) PullRequestsList(repoSlug string) (PullRequests, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests/"
	res, err := b.execute("GET", urlStr, "")
	if err != nil {
		return PullRequests{}, err
	}

	var pullRequests PullRequests
	err = mapstructure.Decode(res, &pullRequests)
	if err != nil {
		return PullRequests{}, err
	}
	return pullRequests, nil
}

// PullRequestCommits returns commits in pull request by pull request id and repository slug
func (b *Bitbucket) PullRequestCommits(repoSlug, prID string) (Commits, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/pullrequests/" + prID + "/commits"
	res, err := b.execute("GET", urlStr, "")
	if err != nil {
		return Commits{}, err
	}

	var commits Commits
	err = mapstructure.Decode(res, &commits)
	if err != nil {
		return Commits{}, err
	}
	return commits, nil
}

// CommitsDiff returns files diff of commits by repository slug and commit hash
func (b *Bitbucket) CommitsDiffStats(repoSlug, spec string) (DiffStats, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/diffstat/" + spec
	res, err := b.execute("GET", urlStr, "")
	if err != nil {
		return DiffStats{}, err
	}

	var diffStats DiffStats
	err = mapstructure.Decode(res, &diffStats)
	if err != nil {
		return DiffStats{}, err
	}
	return diffStats, nil
}

// CommitsDiff returns files diff of commits by repository slug and commit hash
func (b *Bitbucket) SrcFile(repoSlug, spec, path string) (string, error) {
	urlStr := b.Url + "/repositories/" + b.Owner + "/" + repoSlug + "/src/" + spec + "/" + path
	res, err := b.execute("GET", urlStr, "")
	if err != nil {
		return "", err
	}
	file := fmt.Sprintf("%v", res)
	return file, nil
}

// FillCache fill cache commits for searching new migrations
func (b *Bitbucket) FillCache() {
	repositories, err := b.RepositoriesList()
	if err != nil {
		logrus.Panic(err)
	}

	var allPullRequests PullRequests
	for _, repository := range repositories.Values {
		pullRequests, err := b.PullRequestsList(repository.Name)
		if err != nil {
			logrus.Panic(err)
		}
		for _, pullRequest := range pullRequests.Values {
			if pullRequest.State == "OPEN" {
				allPullRequests.Values = append(allPullRequests.Values, pullRequest)
			}
		}
	}
	for _, pullRequest := range allPullRequests.Values {
		commits, err := b.PullRequestCommits(pullRequest.Source.Repository.Name, strconv.FormatInt(pullRequest.ID, 10))
		if err != nil {
			logrus.Panic(err)
		}
		for _, commit := range commits.Values {
			b.AllCache[pullRequest.Source.Repository.Name] = append(
				b.AllCache[pullRequest.Source.Repository.Name],
				map[int64]string{pullRequest.ID: commit.Hash},
			)
		}
	}
}

//TODO убрать дублирование кода

// MigrationMessages returns slice of all miigration files
func (b *Bitbucket) MigrationMessages() ([]string, error) {
	repositories, err := b.RepositoriesList()
	if err != nil {
		return nil, err
	}

	var allPullRequests PullRequests
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

	var files []string
	newCache := make(map[string][]map[int64]string)
	for _, pullRequest := range allPullRequests.Values {
		commits, err := b.PullRequestCommits(pullRequest.Source.Repository.Name, strconv.FormatInt(pullRequest.ID, 10))
		if err != nil {
			logrus.Panic(err)
		}
		for _, commit := range commits.Values {
			diffStats, err := b.CommitsDiffStats(commit.Repository.Name, commit.Hash)
			if err != nil {
				return nil, err
			}

			newCache[pullRequest.Source.Repository.Name] = append(
				newCache[pullRequest.Source.Repository.Name],
				map[int64]string{pullRequest.ID: commit.Hash},
			)
			func() {
				for _, commits := range b.AllCache[commit.Repository.Name] {
					if commit.Hash == commits[pullRequest.ID] {
						return
					}
				}
				logrus.Debug("New!")
				for _, diffStat := range diffStats.Values {
					if strings.Contains(diffStat.New.Path, ".sql") {
						logrus.Debug(diffStat.New.Path)
						file, err := b.SrcFile(commit.Repository.Name, commit.Hash, diffStat.New.Path)
						if err != nil {
							logrus.Panic(err)
						}
						files = append(files, pullRequest.Source.Branch.Name+"\n"+file)
					}
				}
			}()
		}
	}
	b.AllCache = newCache
	return files, nil
}
