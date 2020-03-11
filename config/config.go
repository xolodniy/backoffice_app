package config

import (
	"fmt"
	"time"

	"github.com/evalphobia/logrus_sentry"
	"github.com/getsentry/raven-go"
	"github.com/jinzhu/configor"
	"github.com/sirupsen/logrus"
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
		DailyWorkersWorkedTime         Report
		WeeklyWorkersWorkedTime        Report
		ReportClosedSubtasks           Report
		ReportAfterSecondReviewAll     Report
		ReportAfterSecondReviewBE      Report
		ReportAfterSecondReviewFE      Report
		EmployeesExceededTasks         Report
		ReportSlackSpaceEnding         Report
		ReportGitMigrations            Report
		ReportSprintStatus             Report
		ReportClarificationIssues      Report
		Report24HoursReviewIssues      Report
		ReportGitAnsibleChanges        Report
		DailyWorkersLessWorkedMessage  Report
		WeeklyReportOverworkedIssues   Report
		ReportEpicClosedIssues         Report
		ReportLowPriorityIssuesStarted Report
		CheckNeedReplyMessages         Report
		CheckLowerPriorityBlockers     Report
		SendReminders                  Report
		ReportForgottenPRs             Report
		ReportForgottenBranches        Report
	}
	Database
	Amplify
	Telegram struct {
		ReleaseBotAPIKey string
	}
	Users []User
	Sentry
}

// Jira is template to storing jira configuration
type Jira struct {
	Auth struct {
		Username string
		Token    string
	}
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
	BotIDs         []string
	IgnoreList     []string
	Employees      struct {
		DirectorOfCompany string
		ProjectManager    string
		ArtDirector       string
		TeamLeaderBE      string
		TeamLeaderFE      string
		TeamLeaderDevOps  string
		BeTeam            []string
		FeTeam            []string
		Design            []string
		QATeam            []string
		DevOps            []string
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

// Amplify struct for resending amplify messages
type Amplify struct {
	NotifyChannelID string
	ChannelStag     string
	ChannelProd     string
	Mention         []string
}

// Sentry cloud service for accumulating logs from logrus && gin
type Sentry struct {
	EnableSentry        *bool
	DSN                 string
	LogLevel            string
	LoggingHTTPRequests *bool
	Env                 string
}

type User map[string]string

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
	configureSentry(config.Sentry)
	return &config
}

// checkConfig check general auth configuration
func (config *Main) checkConfig() error {

	if config.Jira.Auth.Username == "" {
		return fmt.Errorf("Jira Username configuration field is not set. Please set it in configuration file «config/config.yml»")
	}
	if config.Jira.Auth.Token == "" {
		return fmt.Errorf("Jira Token configuration field is not set. Please set it in configuration file «config/config.yml»")
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

func configureSentry(config Sentry) {
	if !*config.EnableSentry {
		return
	}
	if config.DSN == "" {
		logrus.Fatal("sentry.dsn must be specified for using sentry")
	}

	if err := raven.SetDSN(config.DSN); err != nil {
		logrus.WithError(err).Fatal("applying sentry dsn was failed, please recheck that it is valid")
	}
	raven.SetEnvironment(config.Env)
	// gather logrus levels which should be sent to sentry.vpe.ninja/
	var sentryLevels []logrus.Level
	// from most critical level to trace
	for _, level := range logrus.AllLevels {
		sentryLevels = append(sentryLevels, level)

		if config.LogLevel == level.String() {
			hook, err := logrus_sentry.NewSentryHook(config.DSN, sentryLevels)
			if err != nil {
				logrus.WithError(err).Fatal("can't init sentry")
			}

			// increase default server response timeout in case of long Sentry response
			hook.Timeout = time.Second

			logrus.AddHook(hook)
			return
		}
	}
	logrus.Fatal("invalid sentry.logLevel variable. Available values: ", logrus.AllLevels)
}
