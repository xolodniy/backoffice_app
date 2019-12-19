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
	ID            int
	PullRequestID int
	Title         string
	Author        string
	RepoSlug      string
	Href          string
	LastActivity  time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ForgottenBranch struct {
	ID        int
	Name      string
	Author    string
	RepoSlug  string
	Href      string
	CreatedAt time.Time
}
