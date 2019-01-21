package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"backoffice_app/types"

	"github.com/sirupsen/logrus"
	"net/url"
)

// Slack is main Slack client app implementation
type Slack struct {
	Auth    SlackAuth
	Channel SlackChannel
	APIURL  string
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
	logrus.Debugf("Slack standard message sent:\n %v", message)

	err := slack.postChannelMessage(
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

// SendStandardMessageWithIcon is main message sending method
func (slack *Slack) SendStandardMessageWithIcon(message, channelID, botName string, iconURL string) error {
	logrus.Debugf("Slack standard message with icon sent:\n %v", message)

	err := slack.postChannelMessage(
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

func (slack *Slack) ListFiles(count string) ([]types.ListFilesResponseFile, error) {
	// Prepare request.
	data := url.Values{}
	// For this to work, it should be a user token, not a bot token or something.
	data.Set("token", slack.Auth.InToken)
	data.Set("count", count)

	u, _ := url.ParseRequestURI(slack.APIURL)
	u.Path = "/api/files.list"
	urlStr := u.String()

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", slack.Auth.OutToken))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	//logrus.Info("Slack request body:", string(body))
	//logrus.Info("Slack response Status:", resp.Status)

	// Process response.
	if err != nil {
		return nil, err
	}
	filesResp := types.ListFilesResponse{}
	if err := json.Unmarshal(body, &filesResp); err != nil {
		return nil, err
	}
	if !filesResp.Ok {
		return nil, fmt.Errorf(filesResp.Error)
	}

	return filesResp.Files, nil
}

func (slack *Slack) DeleteFile(id string) error {
	b, err := json.Marshal(types.DeleteFileMessage{
		Token: slack.Auth.InToken,
		File:  id,
	})
	if err != nil {
		return err
	}

	respBody, err := slack.postJSONMessage("files.delete", b)
	var responseBody struct {
		Ok      bool   `json:"ok"`
		Error   string `json:"error"`
		Warning string `json:"warning"`
	}
	if err := json.Unmarshal(respBody, &responseBody); err != nil {
		return err
	}
	if !responseBody.Ok {
		return fmt.Errorf(responseBody.Error)
	}

	return err
}

func (slack *Slack) postJSONMessage(endpoint string, jsonData []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", slack.APIURL+"/"+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", slack.Auth.OutToken))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	logrus.Debug("Slack request body:", string(jsonData))
	logrus.Debug("Slack response Status:", resp.Status)
	body, _ := ioutil.ReadAll(resp.Body)

	return body, nil
}

func (slack *Slack) sendPOSTMessage(message *types.PostChannelMessage) error {

	b, err := json.Marshal(message)
	if err != nil {
		return err
	}

	respBody, err := slack.postJSONMessage("chat.postMessage", b)
	var responseBody struct {
		Ok      bool   `json:"ok"`
		Error   string `json:"error"`
		Warning string `json:"warning"`
	}
	if err := json.Unmarshal(respBody, &responseBody); err != nil {
		return err
	}
	if !responseBody.Ok {
		return fmt.Errorf(responseBody.Error)
	}

	return err
}

func (slack *Slack) postChannelMessage(text, channelID string, asUser bool, username string, iconURL string) error {
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
