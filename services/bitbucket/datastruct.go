package bitbucket

import (
	"time"
)

// PullRequest struct of pull request from bitbucket
// https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories/%7Busername%7D/%7Brepo_slug%7D/pullrequests#get
type pullRequest struct {
	ID          int    `json:"id"`
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
	Activities []pullRequestActivity `json:"-"`
}

func (pr pullRequest) LastActivityDate() time.Time {
	var lastActivity time.Time
	// find activity date by type (there are 3 types: approve, update, comment)
	for _, activity := range pr.Activities {
		if !activity.Approval.Date.IsZero() && lastActivity.Before(activity.Approval.Date) {
			lastActivity = activity.Approval.Date
		}
		if !activity.Update.Date.IsZero() && lastActivity.Before(activity.Update.Date) {
			lastActivity = activity.Update.Date
		}
		if !activity.Comment.CreatedOn.IsZero() && lastActivity.Before(activity.Comment.CreatedOn) {
			if activity.Comment.CreatedOn.Before(activity.Comment.UpdatedOn) {
				lastActivity = activity.Comment.UpdatedOn
				continue
			}
			lastActivity = activity.Comment.CreatedOn
		}
	}
	return lastActivity
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
	FullName  string  `json:"full_name"`
	Name      string  `json:"name"`
	Website   string  `json:"website"`
	Owner     owner   `json:"owner"`
	Scm       string  `json:"scm"`
	Slug      string  `json:"slug"`
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
	Parents []struct {
		Type string `json:"type"`
		Hash string `json:"hash"`
	} `json:"parents"`
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

// PullRequestCreateInfo struct for create PR
// https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories/%7Busername%7D/%7Brepo_slug%7D/pullrequests#post
type PullRequestCreateInfo struct {
	Title  string `json:"title"`
	Source struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
	} `json:"source"`
	Destination struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
	} `json:"destination"`
}

// RepoPushPayload struct for webhook about push event of repository
// https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-entity_repository (Repository events -> Push)
type RepoPushPayload struct {
	Actor      owner      `json:"actor"`
	Repository repository `json:"repository"`
	Push       struct {
		Changes []struct {
			New struct {
				Type   string `json:"type"`
				Name   string `json:"name"`
				Target struct {
					Type    string    `json:"type"`
					Hash    string    `json:"hash"`
					Author  owner     `json:"author"`
					Message string    `json:"message"`
					Date    time.Time `json:"date"`
					Parents []struct {
						Type  string `json:"type"`
						Hash  string `json:"hash"`
						Links struct {
							Self struct {
								Href string `json:"href"`
							} `json:"self"`
							HTML struct {
								Href string `json:"href"`
							} `json:"html"`
						} `json:"links"`
					} `json:"parents"`
					Links struct {
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
						HTML struct {
							Href string `json:"href"`
						} `json:"html"`
					} `json:"links"`
				} `json:"target"`
				Links struct {
					Self struct {
						Href string `json:"href"`
					} `json:"self"`
					Commits struct {
						Href string `json:"href"`
					} `json:"commits"`
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
				} `json:"links"`
			} `json:"new"`
			Old struct {
				Type   string `json:"type"`
				Name   string `json:"name"`
				Target struct {
					Type    string    `json:"type"`
					Hash    string    `json:"hash"`
					Author  owner     `json:"author"`
					Message string    `json:"message"`
					Date    time.Time `json:"date"`
					Parents []struct {
						Type  string `json:"type"`
						Hash  string `json:"hash"`
						Links struct {
							Self struct {
								Href string `json:"href"`
							} `json:"self"`
							HTML struct {
								Href string `json:"href"`
							} `json:"html"`
						} `json:"links"`
					} `json:"parents"`
					Links struct {
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
						HTML struct {
							Href string `json:"href"`
						} `json:"html"`
					} `json:"links"`
				} `json:"target"`
				Links struct {
					Self struct {
						Href string `json:"href"`
					} `json:"self"`
					Commits struct {
						Href string `json:"href"`
					} `json:"commits"`
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
				} `json:"links"`
			} `json:"old"`
			Links struct {
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
				Diff struct {
					Href string `json:"href"`
				} `json:"diff"`
				Commits struct {
					Href string `json:"href"`
				} `json:"commits"`
			} `json:"links"`
			Created bool `json:"created"`
			Forced  bool `json:"forced"`
			Closed  bool `json:"closed"`
			Commits []struct {
				Hash    string `json:"hash"`
				Type    string `json:"type"`
				Message string `json:"message"`
				Author  owner  `json:"author"`
				Links   struct {
					Self struct {
						Href string `json:"href"`
					} `json:"self"`
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
				} `json:"links"`
			} `json:"commits"`
			Truncated bool `json:"truncated"`
		} `json:"changes"`
	} `json:"push"`
}

// PullRequestMergedPayload struct for webhook about pull request merge
// https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Merged (Pull Request -> Merged)
type PullRequestMergedPayload struct {
	Actor       owner       `json:"actor"`
	PullRequest pullRequest `json:"pullrequest"`
	Repository  repository  `json:"repository"`
}

// pullRequestActivities
// https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories/%7Busername%7D/%7Brepo_slug%7D/pullrequests/%7Bpull_request_id%7D/activity
type pullRequestActivity struct {
	Comment struct {
		Deleted   bool      `json:"deleted"`
		CreatedOn time.Time `json:"created_on"`
		UpdatedOn time.Time `json:"updated_on"`
	} `json:"comment"`
	Update struct {
		Date time.Time `json:"date"`
	} `json:"update"`
	Approval struct {
		Date time.Time `json:"date"`
	} `json:"approval"`
}

type branch struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Target struct {
		Name   string `json:"name"`
		Author struct {
			Type string `json:"type"`
			User struct {
				DisplayName string `json:"display_name"`
				Username    string `json:"username"`
				UUID        string `json:"uuid"`
			} `json:"user"`
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
		} `json:"author"`
	} `json:"target"`
	Links struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
}
