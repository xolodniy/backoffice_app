package tg_bot

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/Syfaro/telegram-bot-api"
	"github.com/sirupsen/logrus"

	"backoffice_app/common"
	"backoffice_app/model"
	"backoffice_app/services/jira"
)

type ReleaseBot struct {
	ctx    context.Context
	apiKey string
	wg     *sync.WaitGroup
	m      *model.Model
	bot    *tgbotapi.BotAPI
	j      *jira.Jira
}

func NewReleaseBot(ctx context.Context, wg *sync.WaitGroup, apiKey string, m *model.Model, j *jira.Jira) ReleaseBot {
	return ReleaseBot{
		ctx:    ctx,
		wg:     wg,
		apiKey: apiKey,
		m:      m,
		j:      j,
	}
}

func (rb *ReleaseBot) RunBot() {
	rb.wg.Add(1)
	defer rb.wg.Done()

	var err error
	rb.bot, err = tgbotapi.NewBotAPI(rb.apiKey)
	if err != nil {
		logrus.WithError(err).Error("can't run Release bot")
		return
	}

	logrus.Debugf("Authorized on account %s", rb.bot.Self.UserName)

	var ucfg tgbotapi.UpdateConfig = tgbotapi.NewUpdate(0)
	ucfg.Timeout = 60
	updChan, err := rb.bot.GetUpdatesChan(ucfg)
	if err != nil {
		logrus.WithError(err).Error("Cant open updates chan")
		return
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
				rb.answerEmptyCallback(update.CallbackQuery.ID)
				chatID := update.CallbackQuery.Message.Chat.ID
				if update.CallbackQuery.Data != "" {
					rb.processReleaseDetails(chatID, update.CallbackQuery.Data)
				}
				continue
			}
			if update.Message == nil {
				continue
			}
			chatID := update.Message.Chat.ID
			text := update.Message.Text
			nameSuffix := "@" + rb.bot.Self.String()
			// parse commands
			switch {
			default:
				rb.sendText(chatID, helpText)
			case update.Message.Chat.IsPrivate() && text == "/reg",
				text == "/reg"+nameSuffix:
				rb.processRegistration(update.Message.Chat)
			case update.Message.Chat.IsPrivate() && text == "/releases",
				text == "/releases"+nameSuffix:
				rb.showReleases(chatID)
			}
		}

	}
}

func (rb *ReleaseBot) answerEmptyCallback(callbackQueryID string) {
	callback := tgbotapi.NewCallback(callbackQueryID, "")
	callback.ShowAlert = false
	rb.bot.AnswerCallbackQuery(callback)
}

func (rb *ReleaseBot) processRegistration(chat *tgbotapi.Chat) {
	_, err := rb.m.GetRbAuthByTgUserID(chat.ID)
	if err != common.ErrModelNotFound {
		rb.sendText(chat.ID, alreadyRegistered)
		return
	}
	if err := rb.m.CreateRbAuth(model.RbAuth{
		TgUserID:  chat.ID,
		Username:  chat.UserName,
		FirstName: chat.FirstName,
		LastName:  chat.LastName,
		Title:     chat.Title,
		Projects:  []string{},
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		rb.sendText(chat.ID, regFailed)
		return
	}
	rb.sendText(chat.ID, regSuccess)
	// TODO send to pm msg about registration
}

func (rb *ReleaseBot) showReleases(chatID int64) {
	rbAuth, err := rb.m.GetRbAuthByTgUserID(chatID)
	if err != nil {
		rb.sendText(chatID, needToBeRegistered)
		return
	}
	type record struct {
		projectName string
		versionName string
		versionID   string
	}
	respSlice := make([]record, 0)
	for _, projectKey := range rbAuth.Projects {
		versions, err := rb.j.UnreleasedFixVersionsByProjectKey(projectKey)
		if err != nil {
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
	if len(respSlice) == 0 {
		rb.sendText(chatID, noProjectAvailable)
		return
	}
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
}

func (rb *ReleaseBot) processReleaseDetails(chatID int64, releaseIDstr string) {

	releaseID, err := strconv.Atoi(releaseIDstr)
	if err != nil {
		logrus.WithError(err).WithField("releaseIDstr", releaseIDstr).Error("can't convert release id to int")
		rb.sendText(chatID, internalError)
		return
	}
	ver, _, err := rb.j.Version.Get(releaseID)
	if err != nil {
		logrus.WithError(err).WithField("releaseIDstr", releaseIDstr).Error("can't get jira version by id")
		rb.sendText(chatID, internalError)
		return
	}
	project, _, err := rb.j.Project.Get(strconv.Itoa(ver.ProjectID))
	if err != nil {
		logrus.WithError(err).WithField("releaseIDstr", releaseIDstr).Error("cant get project from jira")
		rb.sendText(chatID, internalError)
		return
	}
	// check project access for the user
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
	// prepare response
	releasedStatus := "unreleased"
	if ver.Released {
		releasedStatus = "released"
	}
	issuesCount, unresolvedCount, err := rb.j.VersionIssuesCount(releaseID)
	if err != nil {
		rb.sendText(chatID, internalError)
		return
	}
	percent := (float32(issuesCount-unresolvedCount) / float32(issuesCount)) * 100
	resp := fmt.Sprintf("*%s*\n\nCurrent status: %s\n\nRelease date planned: %s\n\nIssues resolved: %d / %d (%2.0f %%)",
		ver.Name, releasedStatus, ver.ReleaseDate, issuesCount-unresolvedCount, issuesCount, percent)

	msg := tgbotapi.NewMessage(chatID, resp)
	msg.ParseMode = tgbotapi.ModeMarkdown
	rb.sendMsgWithLog(msg)
}
