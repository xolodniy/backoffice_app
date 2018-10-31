package config

import (
	"fmt"

	"backoffice_app/clients"
	"backoffice_app/types"

	"github.com/jinzhu/configor"
)

// Main is template to storing of all configuration settings needed
type Main struct {
	WorkedTimeSendTime string
	Jira               types.Jira
	Slack              clients.Slack
	Hubstaff           types.Hubstaff
}

func GetConfig(skipFieldsFilledCheck bool) (*Main, error) {
	var config Main
	//configor.New(&configor.Main{Debug: true}).Load(&config, "config/config.yml")
	if err := configor.Load(&config, "config/config.yml"); err != nil {
		return &Main{}, err

	}

	//spew.Dump(config)
	//fmt.Printf("config: %+v\n", config)

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
	//fmt.Printf("config: %+v\n", config)

	return nil
}

// SlackAuth is template to store authorization tokens to send and receive messages from Slack and in Slack
type SlackAuth struct {
	InToken  string `default:"someSlackInToken"`
	OutToken string `default:"someSlackOutToken"`
}

// SlackChannel is template for user name and ID of the channel to send message there
type SlackChannel struct {
	BotName string `default:"someSlackBotName"`
	ID      string `default:"someSlackChannelID"`
}

// SlackToken is template for Slack token which is using in authorization process
type SlackToken struct {
	slackToken string
}
