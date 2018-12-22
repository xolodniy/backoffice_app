package config

import (
	"fmt"

	"github.com/andygrunwald/go-jira"
	"github.com/jinzhu/configor"
)

// Main is template to storing of all configuration settings needed
type Main struct {
	LogLevel                        string
	GinPort                         string
	GinDebugMode                    bool
	DailyReportCronTime             string
	WeeklyReportCronTime            string
	TaskTimeExceedionReportCronTime string
	GitToken                        string
	Jira                            struct {
		Auth   jira.BasicAuthTransport
		APIUrl string
	}
	Slack struct {
		Auth struct {
			InToken  string
			OutToken string
		}
		Channel struct {
			BotName         string
			BackOfficeAppID string
			MigrationsID    string
		}
		APIUrl string
	}
	Hubstaff struct {
		APIUrl string
		Auth   struct {
			Token    string
			AppToken string

			Login    string
			Password string
		}
		OrgsID int64
	}
}

// GetConfig return config parsed from config/config.yml
func GetConfig(skipFieldsFilledCheck bool) (*Main, error) {
	var config Main
	if err := configor.Load(&config, "/etc/backoffice_app/config.yml"); err != nil {
		return &Main{}, err

	}

	if !skipFieldsFilledCheck {
		if err := config.checkConfig(); err != nil {
			return &Main{}, fmt.Errorf("Error on config checking: %+v", err)
		}
	}

	return &config, nil
}

func (config *Main) checkConfig() error {

	if config.Jira.Auth.Username == "" {
		return fmt.Errorf("Jira Username configuration field is not set. Please set it in configuration file «config/config.yml».")
	}
	if config.Jira.Auth.Password == "" {
		return fmt.Errorf("Jira Password configuration field is not set. Please set it in configuration file «config/config.yml».")
	}

	if config.Slack.Auth.InToken == "" {
		return fmt.Errorf("Slack InToken configuration field is not set. Please set it in configuration file «config/config.yml».")
	}
	if config.Slack.Auth.OutToken == "" {
		return fmt.Errorf("Slack OutToken configuration field is not set. Please set it in configuration file «config/config.yml».")
	}

	if config.Hubstaff.Auth.Token == "" {
		return fmt.Errorf("Hubstaff Auth Token is not specified. You can obtain it with \"obtain-hubstaff-token\" option, and then please set it in configuration file «config/config.yml».")
	}

	return nil
}
