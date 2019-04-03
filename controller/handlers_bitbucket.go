package controller

import (
	"net/http"

	"backoffice_app/services/bitbucket"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func (c *Controller) commitPushed(ctx *gin.Context) {
	repoPushPayload := bitbucket.RepoPushPayload{}
	err := ctx.ShouldBindJSON(&repoPushPayload)
	if err != nil {
		ctx.String(http.StatusBadRequest, "error")
		logrus.WithError(err).Error("can't bind json from bitnucket webhook")
		return
	}
	go c.App.CreateBranchPullRequest(repoPushPayload)
	ctx.JSON(http.StatusOK, gin.H{"result": "ok"})
}
