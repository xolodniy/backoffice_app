// Check that users are reacted on mention and send additional message if not
// next reactions should be accepted:
// - message in tread bellow
// - reaction emoji on message with mention
// user who now AFK or vacation should be notified when will available
package reports

import (
	"fmt"
	"strings"
	"time"

	"backoffice_app/common"
	"backoffice_app/config"
	"backoffice_app/model"
	"backoffice_app/services/slack"
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
	latestUnix := time.Now().Add(-24 * time.Hour).Unix()
	oldestUnix := time.Now().Add(-23 * time.Hour).Unix()
	channelsList, err := nrm.slack.ChannelsList()
	if err != nil {
		return
	}
	for _, channel := range channelsList {
		if !channel.IsActual() {
			continue
		}
		channel.RemoveMembers(nrm.config.BotIDs)
		channelMessages, err := nrm.slack.ChannelMessageHistory(channel.ID, oldestUnix, latestUnix)
		if err != nil {
			return
		}
		for _, channelMessage := range channelMessages {
			nrm.notifyMentionedUsersIfNeed(channel, channelMessage)
		}
	}

	// Send snoozed reminders for users whose was vacation & afk
	nrm.SendReminders()
}

func (nrm *NeedReplyMessages) notifyMentionedUsersIfNeed(channel slack.Channel, message slack.Message) {
	notifications := make(map[string]slack.Message)

	if strings.Contains(message.Text, "<!channel>") {
		for _, user := range channel.Members {
			notifications[user] = message
		}
	}
	for _, user := range channel.Members {
		if strings.Contains(message.Text, user) {
			notifications[user] = message
		}
	}
	for _, user := range message.ReactedUsers() {
		delete(notifications, user)
	}

	for _, reply := range message.Replies {
		delete(notifications, reply.User)

		replyMessage, err := nrm.slack.ChannelMessage(channel.ID, reply.Ts)
		if err != nil {
			return
		}
		for _, user := range channel.Members {
			if strings.Contains(replyMessage.Text, user) {
				notifications[user] = replyMessage
			}
		}
		for _, user := range replyMessage.ReactedUsers() {
			delete(notifications, user)
		}
	}

	for user, message := range notifications {
		if message.IsMessageFromBot() {
			delete(notifications, user)
		}
	}

	afkUsers, _ := nrm.getAfkUsersIDs()
	for _, user := range afkUsers {
		if m, ok := notifications[user]; ok {
			nrm.model.CreateReminder(model.Reminder{
				UserID:     user,
				Message:    fmt.Sprintf("<@%s> %s", user, m.Ts),
				ChannelID:  channel.ID,
				ThreadTs:   message.Ts,
				ReplyCount: message.ReplyCount,
			})
			delete(notifications, user)
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
		nrm.slack.SendToThread(users+"\n>"+template, channel.ID, message.Ts)
	}
}

// sendMessageToNotReactedUsers sends messages to not reacted users for CheckNeedReplyMessages method
func (nrm *NeedReplyMessages) sendMessageToNotReactedUsers(channelMessage slack.Message, channel slack.Channel, repliedUsers []string) {
	reactedUsers := channelMessage.ReactedUsers()
	var notReactedUsers []string
	for _, member := range channel.Members {
		if !common.ValueIn(member, reactedUsers...) && !common.ValueIn(member, repliedUsers...) && member != channelMessage.User {
			notReactedUsers = append(notReactedUsers, member)
		}
	}
	if len(notReactedUsers) == 0 {
		return
	}
	afkUsers, err := nrm.getAfkUsersIDs()
	if err != nil {
		return
	}
	var message string
	for _, userID := range notReactedUsers {
		if common.ValueIn(userID, afkUsers...) {
			nrm.model.CreateReminder(model.Reminder{
				UserID:     userID,
				Message:    "<@" + userID + "> ",
				ChannelID:  channel.ID,
				ThreadTs:   channelMessage.Ts,
				ReplyCount: channelMessage.ReplyCount,
			})
			continue
		}
		message += "<@" + userID + "> "
	}
	nrm.slack.SendToThread(message+" ^", channel.ID, channelMessage.Ts)
}

// sendMessageToMentionedUsers sends messages to mentioned users for CheckNeedReplyMessages method
func (nrm *NeedReplyMessages) sendMessageToMentionedUsers(channelMessage slack.Message, channel slack.Channel, mentionedUsers map[string]string) {
	if len(mentionedUsers) == 0 {
		return
	}
	afkUsers, err := nrm.getAfkUsersIDs()
	if err != nil {
		return
	}
	messages := make(map[string]string)
	for userID, replyTs := range mentionedUsers {
		replyPermalink, err := nrm.slack.MessagePermalink(channel.ID, replyTs)
		if err != nil {
			return
		}
		if common.ValueIn(userID, afkUsers...) {
			nrm.model.CreateReminder(model.Reminder{
				UserID:     userID,
				Message:    fmt.Sprintf("<@%s> %s", userID, replyPermalink),
				ChannelID:  channel.ID,
				ThreadTs:   channelMessage.Ts,
				ReplyCount: channelMessage.ReplyCount,
			})
			continue
		}
		messages[replyPermalink] += "<@" + userID + "> "
	}
	for messagePermalink, message := range messages {
		nrm.slack.SendToThread(fmt.Sprintf("%s %s", message, messagePermalink), channel.ID, channelMessage.Ts)
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
		message, err := nrm.slack.ChannelMessage(reminder.ChannelID, reminder.ThreadTs)
		if err != nil {
			return
		}
		newReplies := message.Replies[reminder.ReplyCount-1:]
		var wasAnswered bool
		for _, reply := range newReplies {
			if reply.User != reminder.UserID {
				continue
			}
			wasAnswered = true
			break
		}
		if !wasAnswered {
			nrm.slack.SendToThread(reminder.Message, reminder.ChannelID, reminder.ThreadTs)
		}
		if err := nrm.model.DeleteReminder(reminder.ID); err != nil {
			return
		}
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
