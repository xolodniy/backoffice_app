package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/sirupsen/logrus"
)

func (c *Controller) slackLastActivityHandler(ctx *gin.Context) {

	form := struct {
		Token       string `form:"token" binding:"required"`
		ResponseURL string `form:"response_url" binding:"required"`
	}{}
	err := ctx.ShouldBindWith(&form, binding.FormPost)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	//security check
	// TODO: make signing checking https://api.slack.com/docs/verifying-requests-from-slack
	if form.Token != c.Config.Slack.AppTokenIn {
		logrus.Error("Invalid token")
		ctx.String(http.StatusUnauthorized, "Invalid token")
		return
	}
	//should to answer to slack and run goroutine with callback
	ctx.JSON(http.StatusOK, gin.H{
		"text": "Report is preparing. Your request will be processed soon.",
	})
	go c.App.ReportLastActivityCallback(form.ResponseURL)
}
