package types

import "github.com/andygrunwald/go-jira"

// JiraIssueSearchParams is an object used to specifying parameters of issues searching in Jira
type JiraIssueSearchParams struct {
	JQL     string
	Options *jira.SearchOptions
}

// Jira is main implementation of Jira app
type Jira struct {
	IssueSearchParams JiraIssueSearchParams
	Auth              jira.BasicAuthTransport
	APIURL            string
}

// Hubstaff is main implementation of Hubstaff app
type Hubstaff struct {
	APIURL string
	Auth   HubstaffAuth
	OrgsID int64
}

// HubstaffAuth is an object used to specifying parameters of issues searching in Hubstaff
type HubstaffAuth struct {
	Token    string
	AppToken string

	Login    string
	Password string
}

// Attachment used to make append and attachment to a simple message
type Attachment struct {
	Text string `json:"text"`
}

// PostChannelMessageAttachment used to make append and attachment to a PostChannelMessage
type PostChannelMessageAttachment struct {
	Text    string `json:"text"`
	PreText string `json:"pre-text" json:"text"`
}

// PostChannelMessage used to make a message with specifying of more details
type PostChannelMessage struct {
	Token       string                         `json:"token"`
	Channel     string                         `json:"channel"`
	AsUser      bool                           `json:"as_user"`
	Text        string                         `json:"text"`
	Username    string                         `json:"username"`
	Attachments []PostChannelMessageAttachment `json:"attachments"`
	IconURL     string                         `json:"icon_url"`
}

// ListFilesResponse is a response sent for ListFilesMessage request.
type ListFilesResponse struct {
	Ok      bool                    `json:"ok"`
	Error   string                  `json:"error"`
	Warning string                  `json:"warning"`
	Files   []ListFilesResponseFile `json:"files"`
}

// ListFilesResponseFile is a single file in ListFilesResponse.
type ListFilesResponseFile struct {
	ID string `json:"id"`
}

// DeleteFileMessage is a request to delete a file.
type DeleteFileMessage struct {
	Token string `json:"token"`
	File  string `json:"file"`
}

// Message is template to make a simple message
type Message struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments"`
}

// Worker used to store an worker data
type Worker struct {
	Name       string `json:"name"`
	TimeWorked int    `json:"duration"`
}

// Workers used to store workers list
type Workers []Worker

// Organization used to store an organization data
type Organization struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	TimeWorked int64    `json:"duration"`
	Workers    []Worker `json:"users"`
}

// Organizations used to store organizations list
type Organizations []Organization

// AddAttachment used to add Attachement to Slack message
func (m *Message) AddAttachment(text string) *Message {
	m.Attachments = append(m.Attachments, Attachment{text})
	return m
}

// AddAttachment used to add Attachement to Slack message
func (pm *PostChannelMessage) AddAttachment(text string, preText string) *PostChannelMessage {
	pm.Attachments = append(pm.Attachments, PostChannelMessageAttachment{Text: text, PreText: preText})
	return pm
}
