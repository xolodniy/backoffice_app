package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"backoffice_app/common"
	"backoffice_app/config"
	"backoffice_app/types"

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
			TeamLeaderDevOps:  "<@" + config.Employees.TeamLeaderDevOps + ">",
			BeTeam:            config.Employees.BeTeam,
			FeTeam:            config.Employees.FeTeam,
			Design:            config.Employees.Design,
			DevOps:            config.Employees.DevOps,
		},
	}
}

// SendMessage is main message sending method
func (s *Slack) SendMessage(text, channel string) {
	channel = s.checkChannelOnUserRealName(channel)

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
	channel = s.checkChannelOnUserRealName(channel)

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
		logrus.WithError(err).WithFields(logrus.Fields{"url": endpoint}).Error("Can't create http request")
		return nil, common.ErrInternal
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.OutToken))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.WithError(err).WithField("request", req).Error("Can't do http request")
		return nil, common.ErrInternal
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).WithField("request", req).Error("Can't read response body")
		return nil, common.ErrInternal
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
			logrus.WithError(err).WithFields(logrus.Fields{"url": urlStr}).Error("Can't create http request")
			return nil, common.ErrInternal
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		filesResp := FilesResponse{}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logrus.WithError(err).WithField("request", req).Error("Can't do http request")
			return nil, common.ErrInternal
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logrus.WithError(err).WithField("request", req).Error("Can't read response body")
			return nil, common.ErrInternal
		}
		if err := json.Unmarshal(body, &filesResp); err != nil {
			logrus.WithError(err).WithField("res", string(body)).
				Error("can't unmarshal response body")
			return nil, common.ErrInternal
		}
		if !filesResp.Ok {
			logrus.WithField("filesResp", filesResp).Error(filesResp.Error)
			return nil, common.ErrInternal
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
	file := types.DeleteFileMessage{
		Token: s.InToken,
		File:  id,
	}
	b, err := json.Marshal(file)
	if err != nil {
		logrus.WithError(err).WithField("reqBody", file).Error("can't narshal request body")
		return common.ErrInternal
	}
	respBody, err := s.jsonRequest("files.delete", b)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"fileId":       id,
			"responseBody": respBody,
		}).Error("can't delete file")
		return common.ErrInternal
	}
	return nil
}

// UploadFile uploads file to slack channel
func (s *Slack) UploadFile(channel, contentType string, file *bytes.Buffer) error {
	urlStr := s.APIURL + "/files.upload"

	v := url.Values{}
	v.Add("channels", channel)

	req, err := http.NewRequest("POST", urlStr, file)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"url": urlStr}).Error("Can't create http request")
		return common.ErrInternal
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.OutToken))
	req.URL.RawQuery = v.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.WithError(err).WithField("request", req).Error("Can't do http request")
		return common.ErrInternal
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).WithField("request", req).Error("Can't read response body")
		return common.ErrInternal
	}
	var responseBody struct {
		Ok      bool   `json:"ok"`
		Error   string `json:"error"`
		Warning string `json:"warning"`
	}
	if err := json.Unmarshal(respBody, &responseBody); err != nil {
		logrus.WithError(err).WithField("res", string(respBody)).
			Error("can't unmarshal response body for upload file")
		return common.ErrInternal
	}
	if !responseBody.Ok {
		logrus.WithField("response", responseBody).Error(responseBody.Error)
		return common.ErrInternal
	}

	return nil
}

// UsersSlice retrieves slice of all slack members
func (s *Slack) UsersSlice() ([]Member, error) {
	var allUsers []Member
	for i := 0; ; i++ {
		urlStr := fmt.Sprintf("%s/users.list?token=%s&page=%v", s.APIURL, s.InToken, i)

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"url": urlStr}).Error("Can't create http request")
			return []Member{}, common.ErrInternal
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		usersResp := UsersResponse{}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logrus.WithError(err).WithField("request", req).Error("Can't do http request")
			return []Member{}, common.ErrInternal
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logrus.WithError(err).WithField("request", req).Error("Can't read response body")
			return []Member{}, common.ErrInternal
		}
		if err := json.Unmarshal(body, &usersResp); err != nil {
			logrus.WithError(err).WithField("res", string(body)).
				Error("can't unmarshal response body for user slice")
			return []Member{}, common.ErrInternal
		}
		if !usersResp.Ok {
			logrus.WithField("response", usersResp).Error(usersResp.Error)
			return []Member{}, common.ErrInternal
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
func (s *Slack) checkChannelOnUserRealName(channel string) string {
	userNameSlice := strings.Split(channel, " ")
	if len(userNameSlice) > 1 {
		allMembers, err := s.UsersSlice()
		if err != nil {
			return channel
		}
		for _, member := range allMembers {
			if member.Profile.RealName == channel {
				return member.Id
			}
		}
	}
	return channel
}

// ChannelMessageHistory retrieves slice of all slack channel messages by time
func (s *Slack) ChannelMessageHistory(channelID string, latest, oldest int64) ([]Message, error) {
	var (
		channelMessages []Message
		cursor          string
	)
	for i := 0; i <= 500; i++ {
		urlStr := fmt.Sprintf("%s/conversations.history?token=%s&inclusive=true&channel=%s&cursor=%s&latest=%v&oldest=%v&pretty=1",
			s.APIURL, s.InToken, channelID, cursor, latest, oldest)

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"url": urlStr}).Error("Can't create http request")
			return []Message{}, common.ErrInternal
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res := MessagesHistory{}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logrus.WithError(err).WithField("request", req).Error("Can't do http request")
			return []Message{}, common.ErrInternal
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logrus.WithError(err).WithField("request", req).Error("Can't read response body")
			return []Message{}, common.ErrInternal
		}
		if err := json.Unmarshal(body, &res); err != nil {
			logrus.WithError(err).WithField("res", string(body)).
				Error("can't unmarshal response body for channel messages history")
			return []Message{}, common.ErrInternal
		}
		if !res.Ok {
			logrus.WithField("response", res).Error(res.Error)
			return []Message{}, common.ErrInternal
		}
		channelMessages = append(channelMessages, res.Messages...)
		if !res.HasMore {
			break
		}
		cursor = res.ResponseMetadata.NextCursor
		// warning message about big history or endless cycle
		if i == 500 {
			logrus.Warn("Message history exceed count of 500")
		}
		resp.Body.Close()
	}
	return channelMessages, nil
}

// ChannelMessage retrieves slack channel message by ts
func (s *Slack) ChannelMessage(channelID, ts string) (Message, error) {
	logFields := logrus.Fields{"channel": channelID, "ts": ts}

	urlStr := fmt.Sprintf("%s/channels.history?token=%s&inclusive=true&channel=%s&latest=%v&pretty=1&count=1",
		s.APIURL, s.InToken, channelID, ts)
	logFields["urlString"] = urlStr

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		logrus.WithError(err).WithFields(logFields).Error("can't send message to slack channel: create http request filed")
		return Message{}, common.ErrInternal
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqBlob, _ := httputil.DumpRequestOut(req, true)
	logFields["dumpRequest"] = reqBlob

	var res MessagesHistory
	resp, err := http.DefaultClient.Do(req)
	respBlob, _ := httputil.DumpResponse(resp, true)
	logFields["dumpResponse"] = respBlob
	if err != nil {
		logrus.WithError(err).WithFields(logFields).Error("can't send message to slack channel: can't do http request")
		return Message{}, common.ErrInternal
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).WithFields(logFields).Error("can't send message to slack channel: failed to read response body")
		return Message{}, common.ErrInternal
	}
	if err := json.Unmarshal(body, &res); err != nil {
		logrus.WithError(err).WithFields(logFields).Error("can't send message to slack channel: can't unmarshal response body")
		return Message{}, common.ErrInternal
	}
	if !res.Ok {
		logrus.WithFields(logFields).Error("can't send message to slack channel: return wrong response")
		return Message{}, common.ErrInternal
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
		logrus.WithError(err).WithFields(logrus.Fields{"url": urlStr}).Error("Can't create http request")
		return "", common.ErrInternal
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var res struct {
		Ok        bool   `json:"ok"`
		Error     string `json:"error"`
		Permalink string `json:"permalink"`
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.WithError(err).WithField("request", req).Error("Can't do http request")
		return "", common.ErrInternal
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).WithField("request", req).Error("Can't read response body")
		return "", common.ErrInternal
	}
	if err := json.Unmarshal(body, &res); err != nil {
		logrus.WithError(err).WithField("res", string(body)).
			Error("can't unmarshal response body for channel message permalink")
		return "", common.ErrInternal
	}
	if !res.Ok {
		logrus.WithField("response", res).Error(res.Error)
		return "", common.ErrInternal
	}
	return res.Permalink, nil
}

// ChannelsList retrieves slice of channels
func (s *Slack) ChannelsList() ([]Channel, error) {
	var (
		channels []Channel
		cursor   string
	)
	for i := 0; i <= 500; i++ {
		urlStr := fmt.Sprintf("%s/channels.list?token=%s&cursor=%s&pretty=1",
			s.APIURL, s.InToken, cursor)

		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"url": urlStr}).Error("Can't create http request")
			return []Channel{}, common.ErrInternal
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res := ChannelList{}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logrus.WithError(err).WithField("request", req).Error("Can't do http request")
			return []Channel{}, common.ErrInternal
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logrus.WithError(err).WithField("request", req).Error("Can't read response body")
			return []Channel{}, common.ErrInternal
		}
		if err := json.Unmarshal(body, &res); err != nil {
			logrus.WithError(err).WithField("res", string(body)).
				Error("can't unmarshal response body for channels list")
			return []Channel{}, common.ErrInternal
		}
		if !res.Ok {
			logrus.WithField("response", res).Error(res.Error)
			return []Channel{}, common.ErrInternal
		}
		channels = append(channels, res.Channels...)
		if res.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = res.ResponseMetadata.NextCursor
		resp.Body.Close()
	}
	return channels, nil
}

func (s *Slack) SendFile(channel, fileName string) error {
	fileDir, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"filename": fileName,
			"channel":  channel,
		}).Error("can't find file")
		return common.ErrInternal
	}
	filePath := path.Join(fileDir, fileName)
	file, err := os.Open(filePath)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"filename": fileName,
			"channel":  channel,
		}).Error("can't open file")
		return common.ErrInternal
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"filename": fileName,
			"channel":  channel,
		}).Error("can't create multipart from file file")
		return common.ErrInternal
	}
	_, err = io.Copy(part, file)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"filename": fileName,
			"channel":  channel,
		}).Error("can't copy multipart from file")
		return common.ErrInternal
	}
	writer.Close()
	err = s.UploadFile(channel, writer.FormDataContentType(), body)
	if err != nil {
		return err
	}
	file.Close()
	os.Remove(filePath)
	return nil
}
