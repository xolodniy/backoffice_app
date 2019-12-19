package bots

import (
	"context"
	"fmt"
	"sort"
	"strconv"
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
	ctx        context.Context
	apiKey     string
	wg         *sync.WaitGroup
	m          *model.Model
	bot        *tgbotapi.BotAPI
	a          *app.App
	userStatus map[int64]uint8
}

const (
	statusNone = iota
	statusReleaseSelection
)

func NewReleaseBot(ctx context.Context, wg *sync.WaitGroup, apiKey string, m *model.Model, application *app.App) *ReleaseBot {
	return &ReleaseBot{
		ctx:        ctx,
		wg:         wg,
		apiKey:     apiKey,
		m:          m,
		a:          application,
		userStatus: make(map[int64]uint8),
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
			if update.CallbackQuery != nil {
				logrus.Infof("CBC === %+v", update.CallbackQuery)
				chatID := update.CallbackQuery.From.ID
				if rb.userStatus[int64(chatID)] == statusReleaseSelection {
					if update.CallbackQuery.Data != "" {
						rb.processReleaseDetails(int64(chatID), update.CallbackQuery.Data)
						continue
					}
				}
				continue
			}
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

			// requested status in format: project - release
			rb.sendHelp(chatID)
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
	type record struct {
		projectName string
		versionName string
		versionID   string
	}
	respSlice := make([]record, 0)
	for _, projectKey := range rbAuth.Projects {
		versions, err := rb.a.Jira.UnreleasedFixVersionsByProjectKey(projectKey)
		if err != nil {
			logrus.WithError(err).WithField("projectKey", projectKey).Error("can't get versions by project")
			continue
		}
		// get releases by projects names from jira
		for _, version := range versions {
			respSlice = append(respSlice, record{
				projectName: projectKey,
				versionName: version.Name,
				versionID:   version.ID,
			})
		}
	}
	if len(respSlice) > 0 {
		sort.Slice(respSlice, func(i, j int) bool {
			return respSlice[i].projectName < respSlice[j].projectName
		})
		rows := make([][]tgbotapi.InlineKeyboardButton, 0)

		for _, str := range respSlice {
			btn := tgbotapi.NewInlineKeyboardButtonData(str.projectName+"/"+str.versionName, str.versionID)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
		}

		var keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
		resp := tgbotapi.NewMessage(chatID, "Select release please")
		resp.ReplyMarkup = keyboard
		rb.sendMsgWithLog(resp)
		rb.userStatus[chatID] = statusReleaseSelection
	} else {
		rb.sendText(chatID, noProjectAvailable)
		rb.userStatus[chatID] = statusReleaseSelection

	}
}

func (rb *ReleaseBot) sendHelp(chatID int64) {
	defer func() {
		rb.userStatus[chatID] = statusNone
	}()
	rb.sendText(chatID, helpText)
}

func (rb *ReleaseBot) processReleaseDetails(chatID int64, releaseIDstr string) {
	defer func() { rb.userStatus[chatID] = statusNone }()

	releaseID, err := strconv.Atoi(releaseIDstr)
	if err != nil {
		logrus.WithError(err).WithField("releaseIDstr", releaseIDstr).Error("can't convert release id to int")
		rb.sendText(chatID, internalError)
		return
	}
	ver, _, err := rb.a.Jira.Version.Get(releaseID)
	if err != nil {
		logrus.WithError(err).WithField("releaseIDstr", releaseIDstr).Error("can't get jira version by id")
		rb.sendText(chatID, internalError)
		return
	}
	project, _, err := rb.a.Jira.Project.Get(strconv.Itoa(ver.ProjectID))
	if err != nil {
		logrus.WithError(err).WithField("releaseIDstr", releaseIDstr).Error("cant get project from jira")
		rb.sendText(chatID, internalError)
		return
	}

	rbAuth, err := rb.m.GetRbAuthByTgUserID(chatID)
	if err != nil {
		rb.sendText(chatID, internalError)
		return
	}
	hasProjectAccess := false
	for _, projGranted := range rbAuth.Projects {
		if projGranted == project.Key {
			hasProjectAccess = true
			break
		}
	}
	if !hasProjectAccess {
		rb.sendText(chatID, projectAccessDenied)
		return
	}
	releasedStatus := "unreleased"
	if ver.Released {
		releasedStatus = "released"
	}
	issuesCount, unresolvedCount, err := rb.a.Jira.VersionIssuesCount(releaseID)
	if err != nil {
		logrus.WithError(err).WithField("releaseIDstr", releaseIDstr).Error("cant get release counts from jira")
		rb.sendText(chatID, internalError)
		return
	}
	percent := (float32(issuesCount-unresolvedCount) / float32(issuesCount)) * 100
	resp := fmt.Sprintf("*%s*\n\nCurrent status: %s\n\nRelease date planned: %s\n\nIssues resolved: %d / %d (%2.0f %%)",
		ver.Name, releasedStatus, ver.ReleaseDate, (issuesCount - unresolvedCount), issuesCount, percent)

	msg := tgbotapi.NewMessage(chatID, resp)
	msg.ParseMode = tgbotapi.ModeMarkdown

	rb.sendMsgWithLog(msg)
}
