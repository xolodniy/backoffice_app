package config

import (
	"backoffice_app/types"
	"log"

	"backoffice_app/clients"

	"github.com/jinzhu/configor"
)

type Config struct {
	Jira     types.Jira
	Slack    clients.Slack
	HubStaff types.HubStaff
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
