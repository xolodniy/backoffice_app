package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"backoffice_app/config"
	"backoffice_app/types"

	"github.com/sirupsen/logrus"
)

// Slack is main Slack client app implementation
type Slack struct {
	InToken         string `default:"someSlackInToken"`
	OutToken        string `default:"someSlackOutToken"`
	BotName         string
	BackofficeAppID string
	APIURL          string
}

// New creates new slack
func New(config *config.Slack) Slack {
	return Slack{
		InToken:         config.InToken,
		OutToken:        config.OutToken,
		BotName:         config.BotName,
		BackofficeAppID: "#" + config.BackOfficeAppID,
		APIURL:          config.APIURL,
	}
}

// SendMessage is main message sending method
func (s *Slack) SendMessage(text string) {
	var message = &types.PostChannelMessage{
		Token:    s.OutToken,
		Channel:  s.BackofficeAppID,
		AsUser:   false,
		Text:     text,
		Username: s.BotName,
		IconURL:  "",
	}

	jsonMessage, err := json.Marshal(message)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      s.BackofficeAppID,
			"channelBotName": s.BotName,
		}).Error("can't decode to json")
	}
	var responseBody struct {
		Ok      bool   `json:"ok"`
		Error   string `json:"error"`
		Warning string `json:"warning"`
	}

	respBody, err := s.jsonRequest("chat.postMessage", jsonMessage)
	if err := json.Unmarshal(respBody, &responseBody); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      s.BackofficeAppID,
			"channelBotName": s.BotName,
		}).Error("can't encode from json")
	}
	if !responseBody.Ok {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      s.BackofficeAppID,
			"channelBotName": s.BotName,
		}).Error(responseBody.Error)
	}
}

func (s *Slack) jsonRequest(endpoint string, jsonData []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", s.APIURL+"/"+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.OutToken))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	return body, nil
}

func (s *Slack) ListFiles(count string) ([]types.ListFilesResponseFile, error) {
	// Prepare request.
	data := url.Values{}
	// For this to work, it should be a user token, not a bot token or something.
	data.Set("token", s.InToken)
	data.Set("count", count)

	u, _ := url.ParseRequestURI(s.APIURL)
	u.Path = "/api/files.list"
	urlStr := u.String()

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.OutToken))
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

func (s *Slack) DeleteFile(id string) error {
	b, err := json.Marshal(types.DeleteFileMessage{
		Token: s.InToken,
		File:  id,
	})
	if err != nil {
		return err
	}

	respBody, err := s.jsonRequest("files.delete", b)
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
