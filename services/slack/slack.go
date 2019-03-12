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
	InToken        string
	OutToken       string
	BotName        string
	ProjectManager string
	APIURL         string
	TotalVolume    float64
	RestVolume     float64
	Secret         string
	IgnoreList     []string
}

// FilesResponse is struct of file.list answer (https://api.slack.com/methods/files.list)
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

// UsersResponse is struct of users.list answer (https://api.slack.com/methods/users.list)
type UsersResponse struct {
	Ok      bool     `json:"ok"`
	Error   string   `json:"error"`
	Warning string   `json:"warning"`
	Members []Member `json:"members"`
	Paging  struct {
		Count int `json:"count"`
		Total int `json:"total"`
		Page  int `json:"page"`
		Pages int `json:"pages"`
	} `json:"paging"`
}

// Files piece of FilesResponse struct for files api answer
type Files struct {
	ID   string  `json:"id"`
	Size float64 `json:"size"`
}

// Member is user object contains information about a member https://api.slack.com/types/user
type Member struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Profile struct {
		Email string `json:"email"`
	} `json:"profile"`
}

// New creates new slack
func New(config *config.Slack) Slack {
	return Slack{
		InToken:        config.InToken,
		OutToken:       config.OutToken,
		BotName:        config.BotName,
		ProjectManager: "<@" + config.ProjectManager + ">",
		APIURL:         config.APIURL,
		TotalVolume:    config.TotalVolume,
		Secret:         config.Secret,
		RestVolume:     config.RestVolume,
		IgnoreList:     config.IgnoreList,
	}
}

// SendMessage is main message sending method
func (s *Slack) SendMessage(text, channel string) {
	var message = &types.PostChannelMessage{
		Token:   s.OutToken,
		Channel: channel,
		AsUser:  true,
		Text:    text,
	}

	jsonMessage, err := json.Marshal(message)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      channel,
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
			"channelID":      channel,
			"channelBotName": s.BotName,
		}).Error("can't encode from json")
	}
	if !responseBody.Ok {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      channel,
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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	return body, nil
}

// Files returns all files info from slack
func (s *Slack) Files() ([]Files, error) {
	var files []Files
	for i := 0; ; i++ {
		urlStr := fmt.Sprintf("%s/files.list?token=%s&page=%v", s.APIURL, s.InToken, i)

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		filesResp := FilesResponse{}
		resp, err := http.DefaultClient.Do(req)
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

// FilesSize retrieves filez size in Gb
func (s *Slack) FilesSize() (float64, error) {
	files, err := s.Files()
	if err != nil {
		return 0, err
	}
	var sum float64
	for _, file := range files {
		sum += file.Size
	}
	return sum / 1024 / 1024 / 1024, nil
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

// UploadFile uploads file to slack channel
func (s *Slack) UploadFile(channel, contentType string, file *bytes.Buffer) error {
	urlStr := s.APIURL + "/files.upload"

	v := url.Values{}
	v.Add("channels", channel)

	req, err := http.NewRequest("POST", urlStr, file)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.OutToken))
	req.URL.RawQuery = v.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
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

	return nil
}

// UserEmailByName retrieve user email by his name
func (s *Slack) UserIdByEmail(email string) (string, error) {
	for i := 0; ; i++ {
		urlStr := fmt.Sprintf("%s/users.list?token=%s&page=%v", s.APIURL, s.InToken, i)

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		usersResp := UsersResponse{}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(body, &usersResp); err != nil {
			return "", err
		}
		if !usersResp.Ok {
			return "", fmt.Errorf(usersResp.Error)
		}
		for _, member := range usersResp.Members {
			if member.Profile.Email == email {
				return member.Id, nil
			}
		}
		if usersResp.Paging.Pages == i {
			break
		}
	}
	return "", nil
}
