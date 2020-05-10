// Check that users are reacted on mention and send additional message if not
// next reactions should be accepted:
// - message in tread bellow
// - reaction emoji on message with mention
// user who now AFK or vacation should be notified when will available
package reports

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"backoffice_app/common"
	"backoffice_app/config"
	"backoffice_app/model"
	"backoffice_app/services/slack"

	"github.com/sirupsen/logrus"
)

type NeedReplyMessages struct {
	config config.Main
	model  model.Model
	slack  slack.Slack
}

func NewNeedReplyMessages(
	c config.Main,
	m model.Model,
	s slack.Slack,
) NeedReplyMessages {
	return NeedReplyMessages{
		config: c,
		model:  m,
		slack:  s,
	}
}

// CheckNeedReplyMessages check messages in all channels for need to reply on it if user was mentioned
func (nrm *NeedReplyMessages) Run() {
	var (
		dateStart = time.Now().Add(-24 * time.Hour)
		dateEnd   = dateStart.Add(time.Minute)
	)
	for _, channel := range nrm.slack.Channels() {
		if channel.NumMembers < 2 { // exclude extra requests to channels where notifications are excess
			continue
		}
		channelMessages := nrm.slack.ChannelMessageHistory(channel.ID, dateStart, dateEnd)
		if len(channelMessages) > 0 {
			logrus.Tracef("channel %s(%s), messages count: %v", channel.Name, channel.ID, len(channelMessages))
		}
		for _, channelMessage := range channelMessages {
			nrm.notifyMentionedUsersIfNeed(channel, channelMessage)
		}
	}

	// Send snoozed reminders for users whose were on vacation & afk
	nrm.SendReminders()
}

func (nrm *NeedReplyMessages) notifyMentionedUsersIfNeed(channel slack.Channel, message slack.Message) {
	logrus.Tracef("message from user %s: %.100s", message.User, message.Text)
	notifications := make(map[string]slack.Message)

	if strings.Contains(message.Text, "<!channel>") {
		for _, user := range channel.Members() {
			notifications[user] = message
		}
		delete(notifications, message.User)
	}
	for _, user := range channel.Members() {
		if strings.Contains(message.Text, user) {
			notifications[user] = message
		}
	}
	if len(notifications) > 0 {
		logrus.Trace("catch notifications for users: ", reflect.ValueOf(notifications).MapKeys())
	}
	for _, user := range message.ReactedUsers() {
		if _, ok := notifications[user]; ok {
			delete(notifications, user)
			logrus.Tracef("excluded reacted user %s, rest notifications: %v", user, reflect.ValueOf(notifications).MapKeys())
		}
	}

	if message.ReplyCount > 0 { // don't remove this condition, ReplyCount already exist, .Replies() method do request to slack API
		logrus.Trace("go range over message replies")
		for _, reply := range message.Replies()[1:] { // first message is initial message, not reply
			logrus.Tracef("reply from %s: %.100s", reply.User, reply.Text)

			for _, user := range channel.Members() {
				if strings.Contains(reply.Text, user) {
					notifications[user] = reply
					logrus.Tracef("catch new notification for %s", user)
				}
			}
			for _, user := range reply.ReactedUsers() {
				if _, ok := notifications[user]; ok {
					delete(notifications, user)
					logrus.Tracef("excluded reacted user %s, rest notifications: %v", user, reflect.ValueOf(notifications).MapKeys())
				}
			}
			if _, ok := notifications[reply.User]; ok {
				delete(notifications, reply.User)
				logrus.Tracef("excluded reply author, rest notifications: %v", reflect.ValueOf(notifications).MapKeys())
			}
		}
	}

	afkUsers, _ := nrm.getAfkUsersIDs()
	for _, user := range afkUsers {
		if m, ok := notifications[user]; ok {
			nrm.model.CreateReminder(model.Reminder{
				UserID:     user,
				Message:    fmt.Sprintf("<@%s>\n>%s", user, m.Text),
				ChannelID:  channel.ID,
				ThreadTs:   message.Ts,
				ReplyCount: message.ReplyCount,
			})
			delete(notifications, user)
			logrus.Tracef("snoozed notification for user %s, because he is afk", user)
		}
	}

	for len(notifications) > 0 {
		var users, template string
		for user, message := range notifications {
			template = message.Text
			users = "<@" + user + "> "
			delete(notifications, user)
			break
		}
		for user, message := range notifications {
			if message.Text == template {
				users += "<@" + user + "> "
				delete(notifications, user)
			}
		}
		text := users + "\n>" + template
		nrm.slack.SendToThread(text, channel.ID, message.Ts)
		logrus.Tracef("into message (%.50s...) sent notification '%s'", message.Text, text)
	}
}

// SendReminders sends reminders for non afk users
func (nrm *NeedReplyMessages) SendReminders() {
	afkUsers, err := nrm.getAfkUsersIDs()
	if err != nil {
		return
	}
	reminders, err := nrm.model.GetReminders()
	if err != nil {
		return
	}
	for _, reminder := range reminders {
		if common.ValueIn(reminder.UserID, afkUsers...) {
			continue
		}
		logrus.Tracef("found available reminder (%.100s...) for user %s", reminder.Message, reminder.UserID)
		replies := slackMessageFromReminder(reminder).Replies()
		// we remember count of replies when reminder was created
		// maybe user answered already, we should check this
		var wasAnswered bool
		newReplies := replies[reminder.ReplyCount:]
		for _, reply := range newReplies {
			if reply.User == reminder.UserID {
				wasAnswered = true
				logrus.Tracef("user already made answer in tread")
				break
			}
			if common.ValueIn(reminder.UserID, reply.ReactedUsers()...) {
				wasAnswered = true
				logrus.Tracef("user sent reaction in some next reply, so we decide he answer")
				break
			}
		}
		if !wasAnswered {
			logrus.Tracef("send snoozed notification (%.100s) from reminder", reminder.Message)
			nrm.slack.SendToThread(reminder.Message, reminder.ChannelID, reminder.ThreadTs)
		}
		nrm.model.DeleteReminder(reminder.ID)
	}
}

// getAfkUsersIDs retrieves all afk users on vacation or with afk status
func (nrm *NeedReplyMessages) getAfkUsersIDs() ([]string, error) {
	var usersIDs []string
	afkTimers, err := nrm.model.GetAfkTimers()
	if err != nil {
		return []string{}, err
	}
	for _, at := range afkTimers {
		usersIDs = append(usersIDs, at.UserID)
	}
	vacations, err := nrm.model.GetActualVacations()
	if err != nil {
		return []string{}, err
	}
	for _, v := range vacations {
		if common.ValueIn(v.UserID, usersIDs...) {
			continue
		}
		usersIDs = append(usersIDs, v.UserID)
	}
	return usersIDs, nil
}

// manual converting reminder to slack message
// should return minimal info for retrieve message replies
func slackMessageFromReminder(reminder model.Reminder) *slack.Message {
	return &slack.Message{
		Channel: reminder.ChannelID,
		Ts:      reminder.ThreadTs,
	}
}
