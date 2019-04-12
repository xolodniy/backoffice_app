package controller

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"backoffice_app/common"

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
	duration, err := time.ParseDuration(request.Text)
	if err != nil {
		ctx.String(http.StatusOK, "Failed! Duration format failed! Please, type /afk 1h30m")
		return
	}
	if c.App.AfkTimer.UserDurationMap[request.UserId] > 0 {
		ctx.String(http.StatusOK, "Failed! You are AFK already")
		return
	}
	go c.App.StartAfkTimer(duration, request.UserId)
	ctx.JSON(http.StatusOK, fmt.Sprintf("You are now AFK for %.0f minutes", duration.Minutes()))
}

func (c *Controller) afkVacationCheck(ctx *gin.Context) {
	request := struct {
		Challenge string `json:"challenge"`
		Event     struct {
			Subtype  string `json:"subtype"`
			Text     string `json:"text"`
			Ts       string `json:"ts"`
			ThreadTs string `json:"thread_ts"`
			Channel  string `json:"channel"`
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
	err := ctx.ShouldBindWith(&request, binding.FormPost)
	if err != nil {
		ctx.String(http.StatusOK, `Failed! Project key is empty! Please, type /vacation 02.01.1970 02.01.1970 "Your message"`)
		return
	}
	if request.Text == "cancel" {
		err := c.App.CancelVacation(request.UserId)
		if err != nil {
			if err == common.ErrNotFound {
				ctx.String(http.StatusOK, "You have no actived vacation autoreply yet")
				return
			}
			ctx.String(http.StatusOK, err.Error())
			return
		}
		ctx.String(http.StatusOK, "Your vacation autoreply has been cancelled")
		return
	}
	if request.Text == "status" {
		vacation, err := c.App.CheckVacationSatus(request.UserId)
		if err != nil {
			if err == common.ErrNotFound {
				ctx.String(http.StatusOK, "You have no actived vacation autoreply yet")
				return
			}
			ctx.String(http.StatusOK, err.Error())
			return
		}
		ctx.JSON(http.StatusOK, fmt.Sprintf(`You have registered vacation autoreply from %s to %s '%s'`,
			vacation.DateStart.Format("02.01.2006"), vacation.DateEnd.Format("02.01.2006"), vacation.Message))
		return
	}

	message := regexp.MustCompile(`"(.+)?"`).FindStringSubmatch(request.Text)
	splitFn := func(c rune) bool {
		return c == ' '
	}
	textSlice := strings.FieldsFunc(request.Text[:strings.IndexByte(request.Text, '"')], splitFn)
	if len(textSlice) != 2 {
		ctx.String(http.StatusOK, `Failed! Project key is empty! Please, type /vacation 02.01.1970 02.01.1970 "Your message"`)
		return
	}
	err = c.App.SetVacationPeriod(textSlice[0], textSlice[1], message[1], request.UserId)
	if err != nil {
		ctx.JSON(http.StatusOK, fmt.Sprintf(err.Error()))
	}
	ctx.JSON(http.StatusOK, fmt.Sprintf("Your vacation autoreply from %s to %s has registered", textSlice[0], textSlice[1]))
}
