package bitbucket

import "time"

//
type PullRequests struct {
	Pagelen int           `json:"pagelen"`
	Page    int           `json:"page"`
	Size    int           `json:"size"`
	Next    string        `json:"next"`
	Values  []PullRequest `json:"values"`
}

type Repositories struct {
	Pagelen int          `json:"pagelen"`
	Page    int          `json:"page"`
	Size    int          `json:"size"`
	Next    string       `json:"next"`
	Values  []Repository `json:"values"`
}

type Commits struct {
	Pagelen int      `json:"pagelen"`
	Page    int      `json:"page"`
	Size    int      `json:"size"`
	Next    string   `json:"next"`
	Values  []Commit `json:"values"`
}

type DiffStats struct {
	Pagelen int        `json:"pagelen"`
	Page    int        `json:"page"`
	Size    int        `json:"size"`
	Next    string     `json:"next"`
	Values  []DiffStat `json:"values"`
}

type Cache struct {
	Name         string `json:"name"`
	PullRequests []CachePullRequest
}

type CachePullRequest struct {
	ID      int64 `json:"id"`
	Commits []Commit
}

type PullRequest struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	Author      Owner  `json:"author"`
	Source      struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
		Repository Repository `json:"repository"`
	} `json:"source"`
	Destination struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
		Repository Repository `json:"repository"`
	} `json:"destination"`
	MergeCommit struct {
		Hash string `json:"hash"`
	} `json:"merge_commit"`
	Participants      []Owner   `json:"participants"`
	Reviewers         []Owner   `json:"reviewers"`
	CloseSourceBranch bool      `json:"close_source_branch"`
	ClosedBy          Owner     `json:"closed_by"`
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

type Repository struct {
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
	Project   Project `json:"project"`
	FullName  string  `json:"full_name"` //empty
	Name      string  `json:"name"`
	Website   string  `json:"website"`
	Owner     Owner   `json:"owner"`
	Scm       string  `json:"scm"`
	IsPrivate bool    `json:"is_private"`
}

type Project struct {
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

type Owner struct {
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

type Commit struct {
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

type DiffStat struct {
	Status       string `json:"status"`
	Type         string `json:"type"`
	LinesRemoved string `json:"lines_removed"`
	LinesAdded   string `json:"lines_added"`
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
	} `json:"old"`
}
