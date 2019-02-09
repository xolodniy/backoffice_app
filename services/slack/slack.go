package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

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

type FilesResponse struct {
	Ok      bool    `json:"ok"`
	Error   string  `json:"error"`
	Warning string  `json:"warning"`
	Files   []Files `json:"files"`
	Paging  struct {
		Count int `json:"count"`
		Total int `json:"total"`
		Page  int `json:"page"`
		Pages int `json:"pages"`
	} `json:"paging"`
}

type Files struct {
	ID   string  `json:"id"`
	Size float64 `json:"size"`
}

var slackSize = 5.0

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

// jsonRequest func for sending json request for slack
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

// Files returns all files info from slack
func (s *Slack) Files() ([]Files, error) {
	// Prepare request.
	data := url.Values{}
	// For this to work, it should be a user token, not a bot token or something.
	data.Set("token", s.InToken)
	var files []Files
	for i := 0; ; i++ {
		data.Set("page", strconv.Itoa(i))

		u, _ := url.ParseRequestURI(s.APIURL)
		u.Path = "/api/files.list"
		urlStr := u.String() + "?" + data.Encode()

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		client := &http.Client{}
		filesResp := FilesResponse{}
		data.Set("page", strconv.Itoa(i))
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &filesResp); err != nil {
			return nil, err
		}
		if !filesResp.Ok {
			return nil, fmt.Errorf(filesResp.Error)
		}
		for _, file := range filesResp.Files {
			files = append(files, file)
		}
		if filesResp.Paging.Pages == i {
			break
		}
	}
	return files, nil
}

// EmtpySpace retrieves empty space on slack
func (s *Slack) FreeSpace() (float64, error) {
	files, err := s.Files()
	if err != nil {
		return 0, err
	}
	var sum float64
	for _, file := range files {
		sum += file.Size
	}
	free := slackSize - (sum / 1024 / 1024 / 1024)
	return free, nil
}

// DeleteFile deletes file from slack by id
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
