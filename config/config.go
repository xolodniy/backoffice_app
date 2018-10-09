package config

import (
	"log"

	"github.com/andygrunwald/go-jira"
	"github.com/jinzhu/configor"
)

type JiraIssueSearchParams struct {
	JQL     string
	Options *jira.SearchOptions
}
type Jira struct {
	IssueSearchParams JiraIssueSearchParams
	Auth              jira.BasicAuthTransport
}

type SlackAuth struct {
	InToken  string `default:"someSlackInToken"`
	OutToken string `default:"someSlackOutToken"`
}

type SlackChannel struct {
	BotName string `default:"someSlackBotName"`
	ID      string `default:"someSlackChannelID"`
}

type Slack struct {
	Auth    SlackAuth
	Channel SlackChannel
}

type HubStaffAuth struct {
	Token    string
	AppToken string

	Login    string
	Password string
}

type HubStaff struct {
	Auth   HubStaffAuth
	OrgsID int64
}

type Config struct {
	Jira     Jira
	Slack    Slack
	HubStaff HubStaff
}

func GetConfig() Config {
	var config Config
	//configor.New(&configor.Config{Debug: true}).Load(&config, "config/config.yml")
	configor.Load(&config, "config/config.yml")

	var defCfg Config

	if config.Jira.Auth.Username == defCfg.Jira.Auth.Username {
		log.Println("Username configuration field is not set. Please set it in configuration file «config/config.yml».")
	}
	if config.Jira.Auth.Password == defCfg.Jira.Auth.Password {
		log.Println("Password configuration field is not set. Please set it in configuration file «config/config.yml».")
	}

	if config.Slack.Auth.InToken == defCfg.Slack.Auth.InToken {
		log.Println("SlackInToken configuration field is not set. Please set it in configuration file «config/config.yml».")
	}
	if config.Slack.Auth.OutToken == defCfg.Slack.Auth.OutToken {
		log.Println("SlackOutToken configuration field is not set. Please set it in configuration file «config/config.yml».")
	}

	//fmt.Printf("config: %+v\n", config)

	return config
}
