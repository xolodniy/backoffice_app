package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"backoffice_app/types"
)

type Slack struct {
	Auth    types.SlackAuth
	Channel types.SlackChannel
	APIUrl  string
}

func (slack *Slack) postJSONMessage(jsonData []byte) (string, error) {
	var url = slack.APIUrl + "/chat.postMessage"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %slack", slack.Auth.OutToken))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	//fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	//fmt.Println("response Body:", string(body))

	return string(body), nil
}

func (slack *Slack) sendPOSTMessage(message *types.PostChannelMessage) (string, error) {

	b, err := json.Marshal(message)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return "", err
	}

	fmt.Printf("JSON IS %+v:\n", string(b))

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
	_, err := slack.postChannelMessage(
		message,
		slack.Channel.ID,
		false,
		slack.Channel.BotName,
	)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return err
	}
	return nil
}
