package controller

import (
	"net/http"

	"backoffice_app/services/jira"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func (c *Controller) issueUpdated(ctx *gin.Context) {
	webHookBody := struct {
		Issue jira.Issue `json:"issue"`
	}{}
	err := ctx.ShouldBindJSON(&webHookBody)
	if err != nil {
		ctx.String(http.StatusBadRequest, "error")
		logrus.WithError(err).Error("can't bind json answer from jira")
		return
	}
	go c.App.MessageIssueAfterSecondTLReview(webHookBody.Issue)
	ctx.JSON(http.StatusOK, gin.H{"result": "ok"})
}
