package types

// Attachment
type Attachment struct {
	Text string `json:"text"`
}

// Message
type Message struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments"`
}

// AddAttachment
func (m *Message) AddAttachment(text string) *Message {
	m.Attachments = append(m.Attachments, Attachment{text})
	return m
}

// NewMessage
func NewMessage(text string) *Message {
	return &Message{
		Text:        text,
		Attachments: make([]Attachment, 0),
	}
}

// Attachment
type PostChannelMessageAttachment struct {
	Text    string `json:"text"`
	PreText string `json:"pre-text" json:"text"`
}
type PostChannelMessage struct {
	Token       string                         `json:"token"`
	Channel     string                         `json:"channel"`
	AsUser      bool                           `json:"as_user"`
	Text        string                         `json:"text"`
	Username    string                         `json:"username"`
	Attachments []PostChannelMessageAttachment `json:"attachments"`
}

// AddAttachment
func (pm *PostChannelMessage) AddAttachment(text string, preText string) *PostChannelMessage {
	pm.Attachments = append(pm.Attachments, PostChannelMessageAttachment{Text: text, PreText: preText})
	return pm
}

// NewMessage
func NewPostChannelMessage(text string, channel string, asUser bool, username string, token string) *PostChannelMessage {
	return &PostChannelMessage{
		Channel:     channel,
		Text:        text,
		AsUser:      asUser,
		Username:    username,
		Token:       token,
		Attachments: make([]PostChannelMessageAttachment, 0),
	}
}

// SlackToken
type SlackToken struct {
	slackToken string
}
type Project struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	LastActivity string `json:"timezone"` /*"2018-09-05T20:26:44.837Z"*/
	Status       string `json:"status"`
	Description  string `json:"description"`
}
type Projects []Project
