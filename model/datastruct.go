package model

import (
	"time"

	"github.com/lib/pq"
)

// Commit struct of commit cache
type Commit struct {
	ID         int
	Type       string
	Hash       string
	Repository string
	Path       string
	Message    string
	CreatedAt  time.Time
}

// AfkTimer struct of Afk timer
type AfkTimer struct {
	ID        int
	UserID    string
	Duration  string
	UpdatedAt time.Time
}

// Vacation struct of user vacation
type Vacation struct {
	UserID    string
	DateStart time.Time
	DateEnd   time.Time
	Message   string
	CreatedAt time.Time
	UpdatedAt time.Time
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

// RbAuth stores Telegram Release Bot user authorization
type RbAuth struct {
	TgUserID  int64
	Username  string
	FirstName string
	LastName  string
	Title     string
	Projects  pq.StringArray
	UpdatedAt time.Time
}

func (rb RbAuth) TableName() string {
	return "rb_auth"
}

// ForgottenPullRequest struct of forgotten pr notify number
type ForgottenPullRequest struct {
	PullRequestID int
	RepoSlug      string
	CreatedAt     time.Time
}

// ForgottenBranch struct of forgotten branches notify number
type ForgottenBranch struct {
	Name      string
	RepoSlug  string
	CreatedAt time.Time
}

// OnDutyUser struct contains users on duty by team
type OnDutyUser struct {
	ID          int
	SlackUserID string
	Team        string
}

type ProtectedBranch struct {
	ID        int
	Name      string
	Comment   string
	UserID    string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}
