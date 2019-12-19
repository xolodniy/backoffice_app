package model

import (
	"time"

	"github.com/lib/pq"
)

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

// RbAuth stores Telegram Release Bot user authorization
type RbAuth struct {
	TgUserID  int64
	Username  string
	FirstName string
	LastName  string
	Projects  pq.StringArray
	UpdatedAt time.Time
}

func (rb RbAuth) TableName() string {
	return "rb_auth"
}
