package bots

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Syfaro/telegram-bot-api"
	"github.com/sirupsen/logrus"

	"backoffice_app/app"
	"backoffice_app/common"
	"backoffice_app/model"
)

type ReleaseBot struct {
	ctx    context.Context
	apiKey string
	wg     *sync.WaitGroup
	m      *model.Model
	bot    *tgbotapi.BotAPI
	a      *app.App
}

func NewReleaseBot(ctx context.Context, wg *sync.WaitGroup, apiKey string, m *model.Model, application *app.App) *ReleaseBot {
	return &ReleaseBot{
		ctx:    ctx,
		wg:     wg,
		apiKey: apiKey,
		m:      m,
		a:      application,
	}
}

func (rb *ReleaseBot) RunBot() {
	rb.wg.Add(1)
	defer rb.wg.Done()

	var err error
	rb.bot, err = tgbotapi.NewBotAPI(rb.apiKey)
	if err != nil {
		logrus.WithError(err).Error("can't run Release bot")
	}
	rb.bot.Debug = true

	logrus.Debugf("Authorized on account %s", rb.bot.Self.UserName)

	var ucfg tgbotapi.UpdateConfig = tgbotapi.NewUpdate(0)
	ucfg.Timeout = 60
	updChan, err := rb.bot.GetUpdatesChan(ucfg)
	if err != nil {
		logrus.WithError(err).Error("Cant open updates chan")
	}
	rb.processMessages(updChan)
}

func (rb *ReleaseBot) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	rb.sendMsgWithLog(msg)
}

func (rb *ReleaseBot) sendProjectAccessDenied(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, projectAccessDenied)
	rb.sendMsgWithLog(msg)
}

func (rb *ReleaseBot) sendMsgWithLog(msg tgbotapi.Chattable) {
	if _, err := rb.bot.Send(msg); err != nil {
		logrus.WithField("msg", fmt.Sprintf("%+v", msg)).WithError(err).Error("cant sent msg")
	}
}

func (rb *ReleaseBot) processMessages(updChan tgbotapi.UpdatesChannel) {

	for {
		select {
		case <-rb.ctx.Done():
			logrus.Info("Stopping ReleaseBot service...")
			rb.bot.StopReceivingUpdates()
			return
		case update := <-updChan:
			if update.Message == nil {
				logrus.WithField("update", fmt.Sprintf("%+v\n", update)).Warn("not understand req")
				continue
			}

			userName := update.Message.From.UserName
			chatID := update.Message.Chat.ID
			text := update.Message.Text
			logrus.Debugf("[%s] %d %s", userName, chatID, text)
			// parse commands
			if strings.Index(text, "/") == 0 {
				switch {
				case text == "/help":
				case text == "/reg":
					rb.processRegistration(chatID, text)
				case text == "/releases":
					rb.showReleases(chatID)
				}
				continue
			}

			// ping-pong
			reply := text
			msg := tgbotapi.NewMessage(chatID, reply)
			rb.sendMsgWithLog(msg)
		}

	}
}

func (rb *ReleaseBot) processRegistration(chatID int64, query string) {
	_, err := rb.m.GetRbAuthByTgUserID(chatID)
	if err != common.ErrNotFound {
		rb.sendText(chatID, alreadyRegistered)
		return
	}
	if err := rb.m.CreateRbAuth(model.RbAuth{
		TgUserID:  chatID,
		Projects:  []string{},
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		rb.sendText(chatID, regFailed)
		return
	}
	rb.sendText(chatID, regSuccess)

	// TODO send to pm msg about registration
}

func (rb *ReleaseBot) showReleases(chatID int64) {
	rbAuth, err := rb.m.GetRbAuthByTgUserID(chatID)
	if err != nil {
		return
	}
	respSlice := make([]string, 0)
	for _, projectKey := range rbAuth.Projects {
		versions, err := rb.a.Jira.UnreleasedFixVersionsByProjectKey(projectKey)
		if err != nil {
			logrus.WithError(err).WithField("projectKey", projectKey).Error("can't get versions by project")
			continue
		}
		// get releases by projects names from jira
		for _, version := range versions {
			respSlice = append(respSlice, fmt.Sprintf("%s - %s\n", projectKey, version.Name))
		}
	}
	if len(respSlice) > 0 {
		sort.Strings(respSlice)
		resp := ""
		for _, str := range respSlice {
			resp += str
		}
		rb.sendText(chatID, resp)
	} else {
		rb.sendText(chatID, noProjectAvailable)
	}
}
