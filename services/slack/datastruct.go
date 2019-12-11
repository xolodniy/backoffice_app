package slack

import "backoffice_app/common"

// Slack is main Slack client app implementation
type Slack struct {
	InToken     string
	OutToken    string
	BotName     string
	APIURL      string
	TotalVolume float64
	RestVolume  float64
	Secret      string
	IgnoreList  []string
	Employees   Employees
}

// Employees is struct of employees in slack
type Employees struct {
	DirectorOfCompany string
	ProjectManager    string
	ArtDirector       string
	TeamLeaderBE      string
	TeamLeaderFE      string
	TeamLeaderDevOps  string
	BeTeam            []string
	FeTeam            []string
	Design            []string
	DevOps            []string
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
		RealName string `json:"real_name"`
		Email    string `json:"email"`
	} `json:"profile"`
}

// MessagesHistory is message object containts information about messages https://api.slack.com/methods/conversations.history
type MessagesHistory struct {
	Ok               bool      `json:"ok"`
	Error            string    `json:"error"`
	Oldest           string    `json:"oldest"`
	Messages         []Message `json:"messages"`
	HasMore          bool      `json:"has_more"`
	IsLimited        bool      `json:"is_limited"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// Message is object that contains message info https://api.slack.com/events/message
type Message struct {
	BotID           string   `json:"bot_id"`
	Type            string   `json:"type"`
	Channel         string   `json:"channel"`
	User            string   `json:"user"`
	Subtype         string   `json:"subtype"`
	Text            string   `json:"text"`
	Ts              string   `json:"ts"`
	ThreadTs        string   `json:"thread_ts"`
	ReplyCount      int      `json:"reply_count"`
	ReplyUsersCount int      `json:"reply_users_count"`
	LatestReply     string   `json:"latest_reply"`
	ReplyUsers      []string `json:"reply_users"`
	Replies         []struct {
		User string `json:"user"`
		Ts   string `json:"ts"`
	} `json:"replies"`
	Reactions []struct {
		Name  string   `json:"name"`
		Users []string `json:"users"`
		Count int      `json:"count"`
	}
}

// IsMessageFromBot checks if message from bot
func (m Message) IsMessageFromBot() bool {
	return m.Subtype != "" || m.BotID != ""
}

// ReactedUsers retrieves user that react on message
func (m Message) ReactedUsers() []string {
	var reactedUsers []string
	for _, rection := range m.Reactions {
		reactedUsers = append(reactedUsers, rection.Users...)
	}
	return reactedUsers
}

// RepliedUsers retrieves user that replies on message
func (m Message) RepliedUsers() []string {
	var repliedUsers []string
	for _, reply := range m.Replies {
		repliedUsers = append(repliedUsers, reply.User)
	}
	return repliedUsers
}

// ChannelList is chanel list object that contains channels https://api.slack.com/methods/channels.list
type ChannelList struct {
	Ok               bool      `json:"ok"`
	Error            string    `json:"error"`
	Channels         []Channel `json:"channels"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// Channel contains channel info
type Channel struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	IsChannel  bool     `json:"is_channel"`
	IsArchived bool     `json:"is_archived"`
	IsPrivate  bool     `json:"is_private"`
	NumMembers int      `json:"num_members"`
	Members    []string `json:"members"`
}

// IsChannelActual checks if channel is actual
func (ch Channel) IsChannelActual() bool {
	return !ch.IsArchived && ch.IsChannel && ch.NumMembers > 0
}

func (ch *Channel) RemoveBotMembers(bots ...string) {
	for i := len(ch.Members) - 1; i >= 0; i-- {
		if common.ValueIn(ch.Members[i], bots...) {
			ch.Members = append(ch.Members[:i], ch.Members[i+1:]...)
		}
	}
}
