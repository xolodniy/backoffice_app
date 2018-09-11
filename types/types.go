package types

// SlackToken
type SlackToken struct {
	slackToken string
}

// Attachment
type Attachment struct {
	Text string `json:"text"`
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


// Message
type Message struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments"`
}

type Worker struct {
	Name       string `json:"name"`
	TimeWorked int    `json:"duration"`
}

type Workers []Worker

type Organization struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	TimeWorked int64    `json:"duration"`
	Workers    []Worker `json:"users"`
}

type Organizations []Organization

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
