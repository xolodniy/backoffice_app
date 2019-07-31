package controller

import (
	"net/http"

	"backoffice_app/services/jira"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func (c *Controller) issueUpdated(ctx *gin.Context) {
	webHookBody := struct {
		Issue     jira.Issue     `json:"issue"`
		Changelog jira.Changelog `json:"changelog"`
	}{}
	if err := ctx.ShouldBindJSON(&webHookBody); err != nil {
		ctx.String(http.StatusBadRequest, "error")
		logrus.WithError(err).Error("can't bind json answer from jira")
		return
	}
	go c.App.MessageIssueAfterSecondTLReview(webHookBody.Issue)
	go c.App.MoveJiraStatuses(webHookBody.Issue)
	go c.App.ChangeJiraSubtasksInfo(webHookBody.Issue, webHookBody.Changelog)
	ctx.JSON(http.StatusOK, gin.H{"result": "ok"})
}

func (c *Controller) issueStarted(ctx *gin.Context) {
	webHookBody := struct {
		Issue jira.Issue `json:"issue"`
	}{}
	err := ctx.ShouldBindJSON(&webHookBody)
	if err != nil {
		ctx.String(http.StatusBadRequest, "error")
		logrus.WithError(err).Error("can't bind json from jira webhook")
		return
	}
	go c.App.CreateIssueBranches(webHookBody.Issue)
	ctx.JSON(http.StatusOK, gin.H{"result": "ok"})
}
