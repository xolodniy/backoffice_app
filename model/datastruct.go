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
	UpdatedAt  time.Time `json:"updatedAt"`
}

// AfkTimer struct of Afk timer
type AfkTimer struct {
	ID        int       `json:"id"`
	UserId    string    `json:"userId"`
	Duration  string    `json:"userId"`
	UpdatedAt time.Time `json:"updatedAt"`
}
