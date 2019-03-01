package controller

import (
	"net/http"

	"backoffice_app/services/bitbucket"

	"github.com/gin-gonic/gin"
)

func (c *Controller) commitPushed(ctx *gin.Context) {
	repoPushPayload := bitbucket.RepoPushPayload{}
	err := ctx.ShouldBindJSON(&repoPushPayload)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	go c.App.CreateBranchPullRequest(repoPushPayload)
	ctx.JSON(http.StatusOK, gin.H{"result": "ok"})
}
