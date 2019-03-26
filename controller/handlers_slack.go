package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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
