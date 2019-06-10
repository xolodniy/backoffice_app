package controller

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"backoffice_app/common"
	"backoffice_app/types"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/sirupsen/logrus"
)

func (c *Controller) slackCurrentActivityHandler(ctx *gin.Context) {
	form := struct {
		ResponseURL string `form:"response_url" binding:"required"`
	}{}
	err := ctx.ShouldBindWith(&form, binding.FormPost)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	//should to answer to slack and run goroutine with callback
	ctx.JSON(http.StatusOK, gin.H{
		"text": "Report is preparing. Your request will be processed soon.",
	})
	go c.App.ReportCurrentActivityWithCallback(form.ResponseURL)
}

func (c *Controller) sprintReport(ctx *gin.Context) {
	request := struct {
		Text   string `form:"text" binding:"required"`
		UserId string `form:"user_id" binding:"required"`
	}{}
	err := ctx.ShouldBindWith(&request, binding.FormPost)
	if err != nil {
		ctx.String(http.StatusOK, "Failed! Project key is empty! Please, type /sprintreport [project key]")
		return
	}
	go func() {
		err := c.App.ReportSprintsIsuues(request.Text, request.UserId)
		if err != nil {
			c.App.Slack.SendMessage(err.Error(), request.UserId)
		}
	}()
	ctx.JSON(http.StatusOK, "ok, wait for report")
}

func (c *Controller) customReport(ctx *gin.Context) {
	request := struct {
		Text   string `form:"text" binding:"required"`
		UserId string `form:"user_id" binding:"required"`
	}{}
	err := ctx.ShouldBindWith(&request, binding.FormPost)
	if err != nil {
		ctx.String(http.StatusOK, "Failed! Project key is empty! Please, type /custom_report @Name 1970-01-01")
		return
	}
	textSlice := strings.Split(request.Text, " ")
	if len(textSlice) != 2 {
		ctx.String(http.StatusOK, "Failed! Format error! Please, type /custom_report @Name 1970-01-01")
		return
	}
	go func() {
		err := c.App.PersonActivityByDate(textSlice[0], textSlice[1], request.UserId)
		if err != nil {
			c.App.Slack.SendMessage(err.Error(), request.UserId)
		}
	}()
	ctx.JSON(http.StatusOK, "ok, wait for report")
}

func (c *Controller) afkCommand(ctx *gin.Context) {
	request := struct {
		Text   string `form:"text" binding:"required"`
		UserId string `form:"user_id" binding:"required"`
	}{}
	err := ctx.ShouldBindWith(&request, binding.FormPost)
	if err != nil {
		ctx.String(http.StatusOK, "Failed! Duration key is empty! Please, type /afk 1h")
		return
	}
	if request.Text == "stop" && c.App.AfkTimer.UserDurationMap[request.UserId] > 0 {
		c.App.AfkTimer.UserDurationMap[request.UserId] = 0
		ctx.String(http.StatusOK, "AFK timer stopped.")
		return
	}
	duration, err := time.ParseDuration(request.Text)
	if err != nil {
		ctx.String(http.StatusOK, "Failed! Duration format failed! Please, type /afk 1h30m")
		return
	}
	if c.App.AfkTimer.UserDurationMap[request.UserId] > 0 {
		c.App.AfkTimer.Lock()
		c.App.AfkTimer.UserDurationMap[request.UserId] += duration
		c.App.AfkTimer.Unlock()
		ctx.JSON(http.StatusOK, fmt.Sprintf("Timer increased. You are now AFK for %.0f minutes",
			c.App.AfkTimer.UserDurationMap[request.UserId].Minutes()))
		return
	}
	go c.App.StartAfkTimer(duration, request.UserId)
	ctx.JSON(http.StatusOK, fmt.Sprintf("You are now AFK for %.0f minutes", duration.Minutes()))
}

func (c *Controller) afkVacationAmplifyCheck(ctx *gin.Context) {
	request := struct {
		Challenge string `json:"challenge"`
		Event     struct {
			UserName    string                               `json:"username"`
			BotID       string                               `json:"bot_id"`
			Subtype     string                               `json:"subtype"`
			Text        string                               `json:"text"`
			Ts          string                               `json:"ts"`
			ThreadTs    string                               `json:"thread_ts"`
			Channel     string                               `json:"channel"`
			Attachments []types.PostChannelMessageAttachment `json:"attachments"`
		} `json:"event" binding:"required"`
	}{}
	err := ctx.ShouldBindJSON(&request)
	switch {
	case err != nil:
		logrus.WithError(err).Errorf("Can't bind json from slack afk check request")
	case request.Event.Subtype == "message_deleted", request.Event.Subtype == "message_changed":
		break
	case request.Event.ThreadTs != "":
		go c.App.CheckUserAfkVacation(request.Event.Text, request.Event.ThreadTs, request.Event.Channel)
	case request.Event.Text != "":
		go c.App.CheckUserAfkVacation(request.Event.Text, request.Event.Ts, request.Event.Channel)
	}
	if len(request.Event.Attachments) != 0 && request.Event.Subtype == "bot_message" {
		go c.App.CheckAmplifyMessage(request.Event.Channel, request.Event.Attachments)
	}
	ctx.JSON(http.StatusOK, gin.H{"challenge": request.Challenge})
}

func (c *Controller) sprintStatus(ctx *gin.Context) {
	request := struct {
		UserId string `form:"user_id" binding:"required"`
	}{}
	err := ctx.ShouldBindWith(&request, binding.FormPost)
	if err != nil {
		ctx.String(http.StatusOK, "Failed! UserId is empty!")
		return
	}
	go c.App.ReportSprintStatus(request.UserId)
	ctx.JSON(http.StatusOK, "ok, wait for report")
}

func (c *Controller) vacation(ctx *gin.Context) {
	request := struct {
		Text   string `form:"text" binding:"required"`
		UserId string `form:"user_id" binding:"required"`
	}{}
	errWrongFormat := `Failed! Format is wrong! Please, type /vacation 02.01.1970 02.01.1970 "Your message"`
	err := ctx.ShouldBindWith(&request, binding.FormPost)
	if err != nil {
		ctx.String(http.StatusOK, errWrongFormat)
		return
	}
	switch {
	case request.Text == "cancel":
		err := c.App.CancelVacation(request.UserId)
		switch {
		case err == common.ErrNotFound:
			ctx.String(http.StatusOK, "You have no actived vacation autoreply yet")
		case err != nil:
			ctx.String(http.StatusOK, err.Error())
		default:
			ctx.String(http.StatusOK, "Your vacation autoreply has been cancelled")
		}
	case request.Text == "status":
		vacation, err := c.App.CheckVacationSatus(request.UserId)
		switch {
		case err == common.ErrNotFound:
			ctx.String(http.StatusOK, "You have no actived vacation autoreply yet")
		case err != nil:
			ctx.String(http.StatusOK, err.Error())
		default:
			ctx.JSON(http.StatusOK, fmt.Sprintf(`You have registered vacation autoreply from %s to %s '%s'`,
				vacation.DateStart.Format("02.01.2006"), vacation.DateEnd.Format("02.01.2006"), vacation.Message))
		}
	default:
		if !strings.Contains(request.Text, `"`) {
			ctx.String(http.StatusOK, errWrongFormat)
			return
		}
		message := regexp.MustCompile(`"(.+)?"`).FindStringSubmatch(request.Text)
		splitFn := func(c rune) bool {
			return c == ' '
		}
		textSlice := strings.FieldsFunc(request.Text[:strings.IndexByte(request.Text, '"')], splitFn)
		if len(message) != 0 && len(textSlice) == 2 {
			err = c.App.SetVacationPeriod(textSlice[0], textSlice[1], message[1], request.UserId)
			if err != nil {
				ctx.JSON(http.StatusOK, fmt.Sprintf(err.Error()))
				return
			}
			ctx.JSON(http.StatusOK, fmt.Sprintf("Your vacation autoreply from %s to %s has registered", textSlice[0], textSlice[1]))
			return
		}
		ctx.String(http.StatusOK, errWrongFormat)
	}
}
