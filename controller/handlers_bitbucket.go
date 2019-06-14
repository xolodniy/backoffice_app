package controller

import (
	"bytes"
	"io/ioutil"
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

func (c *Controller) pullRequestMerged(ctx *gin.Context) {
	body, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{
			"error": "Fail to authorize",
		})
		ctx.Abort()
		return
	}
	ctx.Request.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(body))) //return response body back
	logrus.Debug(string(body))
	pullRequestMergedPayload := bitbucket.PullRequestMergedPayload{}
	err = ctx.ShouldBindJSON(&pullRequestMergedPayload)
	if err != nil {
		ctx.String(http.StatusBadRequest, "error")
		logrus.WithError(err).Error("can't bind json from bitnucket webhook")
		return
	}
	logrus.Debug(pullRequestMergedPayload)
	go c.App.CheckPullRequestsConflicts(pullRequestMergedPayload)
	ctx.JSON(http.StatusOK, gin.H{"result": "ok"})
}
