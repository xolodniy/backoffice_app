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
	"strconv"
	"strings"
	"time"

	"backoffice_app/common"
	"backoffice_app/config"
	"backoffice_app/types"

	"github.com/sirupsen/logrus"
)

// New creates new slack
func New(config *config.Slack) Slack {
	slack = &Slack{
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
	return *slack
}

// allows package objects to communicate with slack api independently
// maybe it will be usable
// Just for experiment, looking how to make more readable code
var slack *Slack

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
	s.jsonRequest("chat.postMessage", jsonMessage)
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
	s.jsonRequest("chat.postMessage", jsonMessage)
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
	s.jsonRequest("chat.postMessage", jsonMessage)
}

// jsonRequest func for sending json request for slack
func (s *Slack) jsonRequest(endpoint string, jsonData []byte) error {
	logFields := logrus.Fields{
		"endpoint": endpoint,
		"jsonData": string(jsonData),
	}
	req, err := http.NewRequest("POST", s.APIURL+"/"+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		logrus.WithError(err).WithFields(logFields).Error("can't do jsonRequest to slack: Can't create http request")
		return common.ErrInternal
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.OutToken))
	reqDump, _ := httputil.DumpRequestOut(req, true)
	logFields["requestDump"] = string(reqDump)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.WithError(err).WithFields(logFields).Error("can't do jsonRequest to slack: Can't do http request")
		return common.ErrInternal
	}
	defer resp.Body.Close()
	respDump, _ := httputil.DumpResponse(resp, true)
	logFields["responseDump"] = string(respDump)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).WithFields(logFields).Error("can't do jsonRequest to slack: Can't read response body")
		return common.ErrInternal
	}
	var errorResponse struct {
		Ok      bool   `json:"ok"`
		Error   string `json:"error"`
		Warning string `json:"warning"`
	}
	if err := json.Unmarshal(body, &errorResponse); err != nil {
		logrus.WithError(err).WithFields(logFields).Error("can't do jsonRequest to slack: Can't encode from json")
		return common.ErrInternal
	}
	if !errorResponse.Ok {
		logrus.WithError(err).WithFields(logFields).Error("can't do jsonRequest to slack: received error response")
		return common.ErrInternal
	}
	return nil
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
	return s.jsonRequest("files.delete", b)
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
func (s *Slack) ChannelMessageHistory(channelID string, dateStart, dateEnd time.Time) []Message {
	var (
		params = url.Values{
			"token":     {s.OutToken},
			"channel":   {channelID},
			"oldest":    {strconv.FormatInt(dateStart.Unix(), 10)},
			"latest":    {strconv.FormatInt(dateEnd.Unix(), 10)},
			"inclusive": {"true"}, // Include messages with latest or oldest timestamp in results only when either timestamp is specified.

			// Slack says about limit param: Fewer than the requested number of items may be returned, even if the end of the users list hasn't been reached.
			// If it's real case than need a new solution with several requests and cursor
			// https://api.slack.com/methods/conversations.history
			"limit": {"1000"},
		}
		url      = fmt.Sprintf("%s/conversations.history?%s", s.APIURL, params.Encode())
		response struct {
			OK               bool      `json:"ok"`
			Error            string    `json:"error"`
			Messages         []Message `json:"messages"`
			HasMore          bool      `json:"has_more"`
			ResponseMetadata struct {
				NextCursor string `json:"next_cursor"`
			} `json:"response_metadata"`
		}
	)
	if log := get(url, &response); log != nil {
		log.Error("can't get conversations.history from slack")
		return []Message{}
	}
	if !response.OK {
		logrus.WithFields(logrus.Fields{
			"url":   url,
			"error": response.Error},
		).Error("received error response from slack while get conversations history")
		return []Message{}
	}

	// back-capability
	// TODO: get rid of this link
	for i := range response.Messages {
		response.Messages[i].Channel = channelID
	}
	return response.Messages
}

// ChannelMessage retrieves slack channel message by ts
// in a head of returned slice will be initial message
func (message *Message) Replies() []Message {
	if len(message.replies) != 0 {
		return message.replies
	}
	var (
		params = url.Values{
			"token":   {slack.OutToken},
			"channel": {message.Channel},
			"ts":      {message.Ts},
			// Slack says about limit param: Fewer than the requested number of items may be returned, even if the end of the users list hasn't been reached.
			// If it's real case than need a new solution with several requests and cursor
			// https://api.slack.com/methods/conversations.replies
			"limit": {"200"},
		}
		url      = fmt.Sprintf("%s/conversations.replies?%s", slack.APIURL, params.Encode())
		response struct {
			Messages []Message `json:"messages"`
			OK       bool      `json:"ok"`
			Error    string    `json:"error"`
		}
	)

	if log := get(url, &response); log != nil {
		log.Error("can't get message replies")
		return []Message{}
	}
	if !response.OK {
		logrus.WithFields(logrus.Fields{"url": url, "error": response.Error}).Error("received error response from slack while get message replies")
		return []Message{}
	}
	message.replies = response.Messages
	return message.replies
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
// Note that default limit is 100 channels, you should upgrade this method if you have more
func (s *Slack) Channels() []Channel {
	var (
		params = url.Values{
			"token":            {s.OutToken},
			"exclude_archived": {"true"},
		}
		url      = fmt.Sprintf("%s/conversations.list?%s", s.APIURL, params.Encode())
		response struct {
			OK       bool      `json:"ok"`
			Error    string    `json:"error"`
			Channels []Channel `json:"channels"`
		}
	)
	if log := get(url, &response); log != nil {
		log.Error("can't get channels from slack")
		return []Channel{}
	}
	if !response.OK {
		logrus.WithFields(logrus.Fields{"url": url, "error": response.Error}).Error("received error response from slack while get channels")
		return []Channel{}
	}
	return response.Channels
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

func (channel *Channel) Members() []string {
	if len(channel.members) != 0 {
		return channel.members
	}

	// Preload members from slack API
	// Note that default members limit is 100.
	//  If your channel has more members - you should improve it yourself
	params := url.Values{
		"token":   {slack.OutToken},
		"channel": {channel.ID},
	}
	url := fmt.Sprintf("%s/conversations.members?%s", slack.APIURL, params.Encode())
	var response struct {
		OK      bool     `json:"ok"`
		Error   string   `json:"error"`
		Members []string `json:"members"`
	}
	if log := get(url, &response); log != nil {
		log.WithFields(logrus.Fields{"URL": url, "channel": channel.Name}).Error("can't get channel members")
		return []string{}
	}
	if !response.OK {
		logrus.WithField("error_from_slack", response.Error).Error("reseived error response from slack during get channel members")
		return []string{}
	}
	channel.members = response.Members
	return channel.members
}

func get(url string, result interface{}) *logrus.Entry {
	log := logrus.WithFields(logrus.Fields{
		"url":                  url,
		"expected_result_type": fmt.Sprintf("%T", result),
		"trace":                common.GetFrames(),
	})
	response, err := http.DefaultClient.Get(url)
	if err != nil {
		return log.WithError(err).WithField("function_get", "can't do http request")
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return log.WithError(err).WithField("function_get", "can't read response body")
	}
	if err := response.Body.Close(); err != nil {
		log.WithError(err).Error("can't close response body")
	}
	if err := json.Unmarshal(body, result); err != nil {
		return log.WithError(err).WithField("function_get", "can't unmarshal response into the expected result")
	}
	return nil
}
