package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"backoffice_app/config"
	"backoffice_app/types"

	"github.com/davecgh/go-spew/spew"
)

const sendItJustInConsole = false

// Slack is main Slack client app implementation
type Slack struct {
	Auth    config.SlackAuth
	Channel config.SlackChannel
	APIUrl  string
}

func (slack *Slack) postJSONMessage(jsonData []byte) (string, error) {
	req, err := http.NewRequest("POST", slack.APIUrl+"/chat.postMessage", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %slack", slack.Auth.OutToken))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	log.Println("Slack response Status:", resp.Status)
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

	log.Printf("sendPOSTMessage message: \n")
	spew.Dump(message)

	resp, err := slack.postJSONMessage(b)

	return resp, err
}
func (slack *Slack) postChannelMessage(text string, channelID string, asUser bool, username string) (string, error) {
	var msg = &types.PostChannelMessage{
		Token:    slack.Auth.OutToken,
		Channel:  channelID,
		AsUser:   asUser,
		Text:     text,
		Username: username,
	}

	return slack.sendPOSTMessage(msg)
}

//Temporarily added. Will be deleted after basic development stage will be finished.
func (slack *Slack) SendConsoleMessage(message string) error {
	fmt.Println(
		message,
	)
	return nil
}
func (slack *Slack) SendStandardMessage(message string) error {
	if sendItJustInConsole {
		slack.SendConsoleMessage(message)
		return nil
	}

	_, err := slack.postChannelMessage(
		message,
		slack.Channel.ID,
		false,
		slack.Channel.BotName,
	)
	if err != nil {
		return err
	}
	return nil
}
