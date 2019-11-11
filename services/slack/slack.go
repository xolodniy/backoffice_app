package slack

import (
	"backoffice_app/config"
	"backoffice_app/types"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
)

// New creates new slack
func New(config *config.Slack) Slack {
	return Slack{
		InToken:     config.InToken,
		OutToken:    config.OutToken,
		BotName:     config.BotName,
		APIURL:      config.APIURL,
		TotalVolume: config.TotalVolume,
		Secret:      config.Secret,
		RestVolume:  config.RestVolume,
		IgnoreList:  config.IgnoreList,
		Employees: Employees{
			DirectorOfCompany: "<@" + config.Employees.DirectorOfCompany + ">",
			ProjectManager:    "<@" + config.Employees.ProjectManager + ">",
			ArtDirector:       "<@" + config.Employees.ArtDirector + ">",
			TeamLeaderBE:      "<@" + config.Employees.TeamLeaderBE + ">",
			TeamLeaderFE:      "<@" + config.Employees.TeamLeaderFE + ">",
			BeTeam:            config.Employees.BeTeam,
			FeTeam:            config.Employees.FeTeam,
		},
	}
}

// SendMessage is main message sending method
func (s *Slack) SendMessage(text, channel string) {
	channel, err := s.checkChannelOnUserRealName(channel)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      channel,
			"channelBotName": s.BotName,
		}).Error("can't find user in slack")
		return
	}
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
		return
	}
	respBody, err := s.jsonRequest("chat.postMessage", jsonMessage)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      channel,
			"channelBotName": s.BotName,
			"responseBody":   respBody,
		}).Error("can't send message")
	}
}

// SendMessageWithAttachments is sending method with attachments
func (s *Slack) SendMessageWithAttachments(text, channel string, attachments []types.PostChannelMessageAttachment) {
	channel, err := s.checkChannelOnUserRealName(channel)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      channel,
			"channelBotName": s.BotName,
		}).Error("can't find user in slack")
		return
	}
	var message = &types.PostChannelMessage{
		Token:       s.OutToken,
		Channel:     channel,
		AsUser:      true,
		Text:        text,
		Attachments: attachments,
	}

	jsonMessage, err := json.Marshal(message)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      channel,
			"channelBotName": s.BotName,
		}).Error("can't decode to json")
		return
	}
	respBody, err := s.jsonRequest("chat.postMessage", jsonMessage)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      channel,
			"channelBotName": s.BotName,
			"responseBody":   respBody,
		}).Error("can't send message")
	}
}

// SendToThread sends message to thread as answer
func (s *Slack) SendToThread(text, channel, threadId string) {
	var message = &types.PostChannelMessage{
		Token:    s.OutToken,
		Channel:  channel,
		AsUser:   true,
		Text:     text,
		ThreadTs: threadId, // parameter that sends message as answer on other message
	}

	jsonMessage, err := json.Marshal(message)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      channel,
			"channelBotName": s.BotName,
			"ThreadTs":       threadId,
		}).Error("can't decode to json")
	}
	respBody, err := s.jsonRequest("chat.postMessage", jsonMessage)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"msgBody":        text,
			"channelID":      channel,
			"channelBotName": s.BotName,
			"responseBody":   respBody,
		}).Error("can't send message")
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
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var responseBody struct {
		Ok      bool   `json:"ok"`
		Error   string `json:"error"`
		Warning string `json:"warning"`
	}

	if err := json.Unmarshal(body, &responseBody); err != nil {
		logrus.WithError(err).Error("can't encode from json")
	}
	if !responseBody.Ok {
		logrus.WithError(err).Error(responseBody.Error)
	}

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
		if filesResp.Paging.Pages <= i {
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
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"fileId":       id,
			"responseBody": respBody,
		}).Error("can't delete file")
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

// usersSlice retrieves slice of all slack members
func (s *Slack) usersSlice() ([]Member, error) {
	var allUsers []Member
	for i := 0; ; i++ {
		urlStr := fmt.Sprintf("%s/users.list?token=%s&page=%v", s.APIURL, s.InToken, i)

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			return []Member{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		usersResp := UsersResponse{}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return []Member{}, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return []Member{}, err
		}
		if err := json.Unmarshal(body, &usersResp); err != nil {
			return []Member{}, err
		}
		if !usersResp.Ok {
			return []Member{}, fmt.Errorf(usersResp.Error)
		}
		for _, member := range usersResp.Members {
			allUsers = append(allUsers, member)
		}
		if usersResp.Paging.Pages <= i {
			break
		}
	}
	return allUsers, nil
}

// checkChannelOnUserRealName retrieve channel with user id if it user real name
func (s *Slack) checkChannelOnUserRealName(channel string) (string, error) {
	userNameSlice := strings.Split(channel, " ")
	if len(userNameSlice) > 1 {
		allMembers, err := s.usersSlice()
		if err != nil {
			return "", err
		}
		for _, member := range allMembers {
			if member.Profile.RealName == channel {
				return member.Id, nil
			}
		}
	}
	return channel, nil
}

// ChannelMessageHistory retrieves slice of all slack channel messages by time
func (s *Slack) ChannelMessageHistory(channelID string, latest, oldest int64) ([]Message, error) {
	var (
		channelMessages []Message
		cursor          string
	)
	for i := 0; ; i++ {
		urlStr := fmt.Sprintf("%s/conversations.history?token=%s&inclusive=true&channel=%s&cursor=%s&latest=%v&oldest=%v&pretty=1",
			s.APIURL, s.InToken, channelID, cursor, latest, oldest)

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			return []Message{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res := MessagesHistory{}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return []Message{}, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return []Message{}, err
		}
		if err := json.Unmarshal(body, &res); err != nil {
			return []Message{}, err
		}
		if !res.Ok {
			return []Message{}, fmt.Errorf(res.Error)
		}
		channelMessages = append(channelMessages, res.Messages...)
		if !res.HasMore {
			break
		}
		cursor = res.ResponseMetadata.NextCursor
	}
	return channelMessages, nil
}

// ChannelMessage retrieves slack channel message by ts
func (s *Slack) ChannelMessage(channelID, ts string) (Message, error) {
	urlStr := fmt.Sprintf("%s/channels.history?token=%s&inclusive=true&channel=%s&latest=%v&pretty=1&count=1",
		s.APIURL, s.InToken, channelID, ts)

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := MessagesHistory{}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Message{}, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Message{}, err
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return Message{}, err
	}
	if !res.Ok {
		return Message{}, fmt.Errorf(res.Error)
	}
	if len(res.Messages) == 0 {
		return Message{}, nil
	}
	return res.Messages[0], nil
}

// MessagePermalink retrieves slack channel message permalink by ts
func (s *Slack) MessagePermalink(channelID, ts string) (string, error) {
	urlStr := fmt.Sprintf("%s/chat.getPermalink?token=%s&channel=%s&message_ts=%v&pretty=1",
		s.APIURL, s.InToken, channelID, ts)

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var res struct {
		Ok        bool   `json:"ok"`
		Error     string `json:"error"`
		Permalink string `json:"permalink"`
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}
	if !res.Ok {
		return "", fmt.Errorf(res.Error)
	}
	return res.Permalink, nil
}

// ChannelMessageHistory retrieves slice of all slack channel messages by time
func (s *Slack) ChannelsList() ([]Channel, error) {
	var (
		channels []Channel
		cursor   string
	)
	for i := 0; ; i++ {
		urlStr := fmt.Sprintf("%s/channels.list?token=%s&cursor=%s&pretty=1",
			s.APIURL, s.InToken, cursor)

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			return []Channel{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res := ChannelList{}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return []Channel{}, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return []Channel{}, err
		}
		if err := json.Unmarshal(body, &res); err != nil {
			return []Channel{}, err
		}
		if !res.Ok {
			return []Channel{}, fmt.Errorf(res.Error)
		}
		channels = append(channels, res.Channels...)
		if res.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = res.ResponseMetadata.NextCursor
	}
	return channels, nil
}
