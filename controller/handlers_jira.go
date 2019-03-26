package controller

import (
	"net/http"

	"backoffice_app/services/jira"

	"github.com/gin-gonic/gin"
)

func (c *Controller) issueUpdated(ctx *gin.Context) {
	webHookBody := struct {
		Issue jira.Issue `json:"issue"`
	}{}
	err := ctx.ShouldBindJSON(&webHookBody)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	go c.App.MessageIssueAfterSecondReview(webHookBody.Issue)
	ctx.JSON(http.StatusOK, gin.H{"result": "ok"})
}
