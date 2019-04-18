package config

import (
	"fmt"

	"github.com/andygrunwald/go-jira"
	"github.com/jinzhu/configor"
)

// Main is template to storing of all configuration settings needed
type Main struct {
	LogLevel            string
	GinPort             string
	GinDebugMode        bool
	MaxWeekWorkingHours float32
	Jira
	Hubstaff
	Slack
	Bitbucket
	Reports struct {
		DailyWorkersWorkedTime        Report
		WeeklyWorkersWorkedTime       Report
		ReportClosedSubtasks          Report
		ReportAfterSecondReviewAll    Report
		ReportAfterSecondReviewBE     Report
		ReportAfterSecondReviewFE     Report
		EmployeesExceededTasks        Report
		ReportSlackSpaceEnding        Report
		ReportGitMigrations           Report
		ReportSprintStatus            Report
		ReportClarificationIssues     Report
		Report24HoursReviewIssues     Report
		ReportGitAnsibleChanges       Report
		DailyWorkersLessWorkedMessage Report
		WeeklyReportOverworkedIssues  Report
		ReportEpicClosedIssues        Report
	}
	Database
}

// Jira is template to storing jira configuration
type Jira struct {
	Auth   jira.BasicAuthTransport
	APIUrl string
}

// Hubstaff is template to storing hubstaff configuration
type Hubstaff struct {
	APIURL string
	Auth   struct {
		Token    string
		AppToken string

		Login    string
		Password string
	}
	OrgsID int64
}

// Slack is template to storing slack configuration
type Slack struct {
	InToken        string
	OutToken       string
	BotName        string
	ProjectManager string
	ArtDirector    string
	APIURL         string
	TotalVolume    float64
	RestVolume     float64
	Secret         string
	IgnoreList     []string
	Employees      struct {
		DirectorOfCompany string
		ProjectManager    string
		ArtDirector       string
		TeamLeaderBE      string
		TeamLeaderFE      string
		BeTeam            []string
		FeTeam            []string
	}
}

// Bitbucket is template to storing bitbucket configuration
type Bitbucket struct {
	APIUrl string
	Owner  string
	Auth   struct {
		Username string
		Password string
	}
}

// Report struct for cron values with channel for reports
type Report struct {
	Schedule string
	Channel  string
}

// Database configuration
type Database struct {
	Host      string
	Port      int
	User      string
	Password  string
	Name      string
	EnableSSL bool
}

// GetConfig return config parsed from config/config.yml
func GetConfig(skipFieldsFilledCheck bool, path string) *Main {
	var config Main
	if err := configor.Load(&config, path); err != nil {
		panic(err)
	}

	if !skipFieldsFilledCheck {
		if err := config.checkConfig(); err != nil {
			panic(err)
		}
	}

	return &config
}

// checkConfig check general auth configuration
func (config *Main) checkConfig() error {

	if config.Jira.Auth.Username == "" {
		return fmt.Errorf("Jira Username configuration field is not set. Please set it in configuration file «config/config.yml»")
	}
	if config.Jira.Auth.Password == "" {
		return fmt.Errorf("Jira Password configuration field is not set. Please set it in configuration file «config/config.yml»")
	}

	if config.Slack.InToken == "" {
		return fmt.Errorf("Slack InToken configuration field is not set. Please set it in configuration file «config/config.yml»")
	}
	if config.Slack.OutToken == "" {
		return fmt.Errorf("Slack OutToken configuration field is not set. Please set it in configuration file «config/config.yml»")
	}

	if config.Hubstaff.Auth.Token == "" {
		return fmt.Errorf("Hubstaff Auth Token is not specified. You can obtain it with \"obtain-hubstaff-token\" option, and then please set it in configuration file «config/config.yml»")
	}

	return nil
}

// DbConnURL returns string URL, which may be used for connect to postgres database.
func (c *Database) ConnURL() string {
	url := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.Name,
	)
	if !c.EnableSSL {
		url += "?sslmode=disable"
	}
	return url
}
