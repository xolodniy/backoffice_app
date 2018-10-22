package config

import (
	"fmt"

	"backoffice_app/clients"
	"backoffice_app/types"

	"github.com/jinzhu/configor"
)

// Config is template to storing of all configuration settings needed
type Config struct {
	WorkedTimeSendTime string
	Jira               types.Jira
	Slack              clients.Slack
	Hubstaff           types.Hubstaff
}

func GetConfig() (*Config, error) {
	var config Config
	//configor.New(&configor.Config{Debug: true}).Load(&config, "config/config.yml")
	configor.Load(&config, "config/config.yml")

	if config.Jira.Auth.Username == "" {
		return nil, fmt.Errorf("Username configuration field is not set. Please set it in configuration file «config/config.yml».")
	}
	if config.Jira.Auth.Password == "" {
		return nil, fmt.Errorf("Password configuration field is not set. Please set it in configuration file «config/config.yml».")
	}

	if config.Slack.Auth.InToken == "" {
		return nil, fmt.Errorf("SlackInToken configuration field is not set. Please set it in configuration file «config/config.yml».")
	}
	if config.Slack.Auth.OutToken == "" {
		return nil, fmt.Errorf("SlackOutToken configuration field is not set. Please set it in configuration file «config/config.yml».")
	}

	//fmt.Printf("config: %+v\n", config)

	return &config, nil
}
