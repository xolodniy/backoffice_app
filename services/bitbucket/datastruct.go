package bitbucket

import "time"

// HashCache struct of hash commits map
type HashCache struct {
	Repository string
	Path       string
	Message    string
}

// PullRequest struct of pull request from bitbucket
// https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories/%7Busername%7D/%7Brepo_slug%7D/pullrequests#get
type pullRequest struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	Author      owner  `json:"author"`
	Source      struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
		Repository repository `json:"repository"`
	} `json:"source"`
	Destination struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
		Repository repository `json:"repository"`
	} `json:"destination"`
	MergeCommit struct {
		Hash string `json:"hash"`
	} `json:"merge_commit"`
	Participants      []owner   `json:"participants"`
	Reviewers         []owner   `json:"reviewers"`
	CloseSourceBranch bool      `json:"close_source_branch"`
	ClosedBy          owner     `json:"closed_by"`
	Reason            string    `json:"reason"`
	CreatedOn         time.Time `json:"created_on"`
	UpdatedOn         time.Time `json:"updated_on"`
	Links             struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
}

// Repository struct of pull repository from bitbucket
// https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories
type repository struct {
	Type  string `json:"type"`
	Links struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
		Avatar struct {
			Href string `json:"href"`
		} `json:"avatar"`
	} `json:"links"`
	UUID      string  `json:"uuid"`
	Project   project `json:"project"`
	FullName  string  `json:"full_name"` //empty
	Name      string  `json:"name"`
	Website   string  `json:"website"`
	Owner     owner   `json:"owner"`
	Scm       string  `json:"scm"`
	IsPrivate bool    `json:"is_private"`
}

// Project struct of pull project from bitbucket
// https://developer.atlassian.com/bitbucket/api/2/reference/resource/teams/%7Busername%7D/projects/#get
type project struct {
	Type    string `json:"type"`
	Project string `json:"project"`
	UUID    string `json:"uuid"`
	Links   struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
		Avatar struct {
			Href string `json:"href"`
		} `json:"avatar"`
	} `json:"links"`
	Key string `json:"key"`
}

// Owner struct of owner from bitbucket
// https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories
type owner struct {
	Type        string `json:"type"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	UUID        string `json:"uuid"`
	Links       struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
		Avatar struct {
			Href string `json:"href"`
		} `json:"avatar"`
	} `json:"links"`
}

// Commit struct of commit from bitbucket
// https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories/%7Busername%7D/%7Brepo_slug%7D/commit/%7Bnode%7D
type commit struct {
	Type  string `json:"type"`
	Hash  string `json:"hash"`
	Links struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
		Patch struct {
			Href string `json:"href"`
		} `json:"patch"`
		Diff struct {
			Href string `json:"href"`
		} `json:"diff"`
	} `json:"links"`
	Repository struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Uuid     string `json:"uuid"`
	} `json:"repository"`
	Message string `json:"message"`
}

// DiffStat struct of diffStat from bitbucket
// https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories/%7Busername%7D/%7Brepo_slug%7D/diffstat/%7Bspec%7D
type diffStat struct {
	Status       string `json:"status"`
	Type         string `json:"type"`
	LinesRemoved int    `json:"lines_removed"`
	LinesAdded   int    `json:"lines_added"`
	Old          struct {
		Path  string `json:"path"`
		Type  string `json:"type"`
		Links struct {
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"links"`
	} `json:"old"`
	New struct {
		Path  string `json:"path"`
		Type  string `json:"type"`
		Links struct {
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"links"`
	} `json:"new"`
}
