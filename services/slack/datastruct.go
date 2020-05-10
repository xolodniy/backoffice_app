package slack

// Slack is main Slack client app implementation
type Slack struct {
	// TODO: check that token usable for bot.
	//  probably we don't need it
	InToken string

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
	Reactions       []struct {
		Name  string   `json:"name"`
		Users []string `json:"users"`
		Count int      `json:"count"`
	}

	replies []Message
}

// IsMessageFromBot checks if message from bot
func (m Message) IsMessageFromBot() bool {
	return m.Subtype != "" || m.BotID != ""
}

// ReactedUsers retrieves user that react on message
func (m Message) ReactedUsers() []string {
	reactions := make(map[string]struct{})
	for _, reaction := range m.Reactions {
		for _, user := range reaction.Users {
			reactions[user] = struct{}{}
		}
	}
	reactedUsers := make([]string, 0, len(reactions))
	for user := range reactions {
		reactedUsers = append(reactedUsers, user)
	}
	return reactedUsers
}

// Channel contains channel info
type Channel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsChannel  bool   `json:"is_channel"`
	IsArchived bool   `json:"is_archived"`
	IsPrivate  bool   `json:"is_private"`
	NumMembers int    `json:"num_members"`

	members []string
}

// IsActual checks if channel is actual
func (ch Channel) IsActual() bool {
	return !ch.IsArchived && ch.IsChannel && ch.NumMembers > 0
}
