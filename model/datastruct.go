package model

// Commit struct of commit cache
type Commit struct {
	ID         int    `json:"id"`
	Type       string `json:"type"`
	Hash       string `json:"hash"`
	Repository string `json:"repository"`
	Path       string `json:"path"`
	Message    string `json:"message"`
}
