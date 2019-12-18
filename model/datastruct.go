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
	UserID    string    `json:"userID"`
	Duration  string    `json:"duration"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Vacation struct of user vacation
type Vacation struct {
	UserID    string    `json:"userID"`
	DateStart time.Time `json:"dateStart"`
	DateEnd   time.Time `json:"dateEnd"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Reminder struct of user reminders
type Reminder struct {
	ID         int
	UserID     string
	Message    string
	ChannelID  string
	ThreadTs   string
	ReplyCount int
	CreatedAt  time.Time
}
