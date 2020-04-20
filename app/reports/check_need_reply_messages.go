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
	repliedUsers := message.RepliedUsers()

	var replyMessages []slack.Message
	// check for replies of channel message
	for _, reply := range message.Replies {
		replyMessage, err := nrm.slack.ChannelMessage(channel.ID, reply.Ts)
		if err != nil {
			return
		}
		if replyMessage.IsMessageFromBot() {
			continue
		}
		replyMessages = append(replyMessages, replyMessage)
	}
	// check reactions of channel members on message if it contains @channel
	if strings.Contains(message.Text, "<!channel>") {
		nrm.sendMessageToNotReactedUsers(message, channel, repliedUsers)
	}
	var mentionedUsers = make(map[string]string)
	if !message.IsMessageFromBot() {
		reactedUsers := message.ReactedUsers()
		for _, userSlackID := range channel.Members {
			if strings.Contains(message.Text, userSlackID) && mentionedUsers[userSlackID] == "" && !common.ValueIn(userSlackID, reactedUsers...) {
				mentionedUsers[userSlackID] = message.Ts
			}
		}
	}
	// check replies for message and new mentions in replies
	for _, replyMessage := range replyMessages {
		delete(mentionedUsers, replyMessage.User)
		if message.IsMessageFromBot() {
			continue
		}
		// if users reacted we don't send message
		reactedUsers := replyMessage.ReactedUsers()
		for _, userSlackID := range channel.Members {
			if strings.Contains(replyMessage.Text, userSlackID) && mentionedUsers[userSlackID] == "" && !common.ValueIn(userSlackID, reactedUsers...) {
				mentionedUsers[userSlackID] = replyMessage.Ts
			}
		}
	}
	nrm.sendMessageToMentionedUsers(message, channel, mentionedUsers)
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
