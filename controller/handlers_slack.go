package controller

import (
	"net/http"

	"backoffice_app/types"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type LastActivityRequest struct {
	Token       string `form:"token" binding:"required"`
	ResponseUrl string `form:"response_url" binding:"required"`
}

func (c *Controller) slackLastActivityHandler(ctx *gin.Context) {

	var form = LastActivityRequest{}

	err := ctx.Bind(&form)
	if err != nil {
		logrus.WithError(err).Error("Can't bind to the form.")
		ctx.String(http.StatusBadRequest, err.Error())
	}
	// TODO: make signing checking https://api.slack.com/docs/verifying-requests-from-slack
	if form.Token != "t2LZHz5L0rNuCxSkDt07dVzu" {
		logrus.Error("Invalid token")
		ctx.String(http.StatusBadRequest, "Invalid token")
		return
	}
	//
	ctx.JSON(http.StatusOK, struct {
		Text        string             `json:"text"`
		Attachments []types.Attachment `json:"attachments"`
	}{
		Text: "Your request will be processed soon...",
	})
	//
	go c.App.MakeLastActivityReportWithCallback(form.ResponseUrl)
}
