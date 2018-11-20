package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"backoffice_app/types"

	"github.com/sirupsen/logrus"
)

// sendItJustInConsole defines whether the app should send messages just in console, but not in Slack channel (needs while further development work)
const sendItJustInConsole = false

// Slack is main Slack client app implementation
type Slack struct {
	Auth    SlackAuth
	Channel SlackChannel
	APIUrl  string
}

// SlackAuth is Slack Authorization data storage used for API and Webhook requests
type SlackAuth struct {
	InToken  string `default:"someSlackInToken"`
	OutToken string `default:"someSlackOutToken"`
}

// SlackChannel is template for user name and BackOfficeAppID of the channel to send message there
type SlackChannel struct {
	BotName string
	ID      string
}

// SendStandardMessage is main message sending method
func (slack *Slack) SendStandardMessage(message, channelID, botName string) error {
	if sendItJustInConsole {
		slack.SendConsoleMessage(message)
		return nil
	}

	_, err := slack.postChannelMessage(
		message,
		channelID,
		false,
		botName,
		"",
	)
	if err != nil {
		return err
	}
	return nil
}

// SendStandardMessage is main message sending method
func (slack *Slack) SendStandardMessageWithIcon(message, channelID, botName string, iconURL string) error {
	if sendItJustInConsole {
		slack.SendConsoleMessage(message)
		return nil
	}

	_, err := slack.postChannelMessage(
		message,
		channelID,
		false,
		botName,
		iconURL,
	)
	if err != nil {
		return err
	}
	return nil
}

func (slack *Slack) postJSONMessage(jsonData []byte) (string, error) {
	req, err := http.NewRequest("POST", slack.APIUrl+"/chat.postMessage", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", slack.Auth.OutToken))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	logrus.Info("Slack request body:", string(jsonData))
	logrus.Info("Slack response Status:", resp.Status)
	//fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)

	var responseBody struct {
		Ok      bool   `json:"ok"`
		Error   string `json:"error"`
		Warning string `json:"warning"`
	}

	if err := json.Unmarshal(body, &responseBody); err != nil {
		return "", err
	}

	if !responseBody.Ok {
		return "", fmt.Errorf(responseBody.Error)
	}

	return string(body), nil
}
func (slack *Slack) sendPOSTMessage(message *types.PostChannelMessage) (string, error) {

	b, err := json.Marshal(message)
	if err != nil {
		return "", err
	}

	//log.Printf("sendPOSTMessage message: \n")
	//spew.Dump(message)

	resp, err := slack.postJSONMessage(b)

	return resp, err
}

func (slack *Slack) postChannelMessage(text, channelID string, asUser bool, username string, iconURL string) (string, error) {
	var msg = &types.PostChannelMessage{
		Token:    slack.Auth.OutToken,
		Channel:  channelID,
		AsUser:   asUser,
		Text:     text,
		Username: username,
		IconURL:  iconURL,
	}

	return slack.sendPOSTMessage(msg)
}

// SendConsoleMessage instead of message to Slack. Used while testing or development.
func (slack *Slack) SendConsoleMessage(message string) error {
	fmt.Println(
		message,
	)
	return nil
}
