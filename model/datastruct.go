package model

import "time"

// Commit struct of commit cache
type Commit struct {
	ID         int       `json:"id"`
	Type       string    `json:"type"`
	Hash       string    `json:"hash"`
	Repository string    `json:"repository"`
	Path       string    `json:"path"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"createdAt"`
}

// AfkTimer struct of Afk timer
type AfkTimer struct {
	ID        int       `json:"id"`
	UserId    string    `json:"userId"`
	Duration  string    `json:"userId"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Vacation struct of user vacation
type Vacation struct {
	UserId    string    `json:"userId"`
	DateStart time.Time `json:"dateStart"`
	DateEnd   time.Time `json:"dateEnd"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ForgottenPullRequest struct {
	ID            int       `json:"id"`
	PullRequestID int64     `json:"pullRequestId"`
	Title         string    `json:"title"`
	Author        string    `json:"author"`
	RepoSlug      string    `json:"repoSlug"`
	Href          string    `json:"href"`
	LastActivity  time.Time `json:"lastActivity"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type ForgottenBranch struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Author    string    `json:"author"`
	RepoSlug  string    `json:"repoSlug"`
	Href      string    `json:"href"`
	CreatedAt time.Time `json:"createdAt"`
}
