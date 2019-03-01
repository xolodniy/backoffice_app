package controller

import (
	"bytes"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"

	"backoffice_app/services/bitbucket"

	"github.com/gin-gonic/gin"
)

func (c *Controller) commitPushed(ctx *gin.Context) {
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
	repoPushPayload := bitbucket.RepoPushPayload{}
	err = ctx.ShouldBindJSON(&repoPushPayload)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	go c.App.CreateBranchPullRequest(repoPushPayload)
	ctx.JSON(http.StatusOK, gin.H{"result": "ok"})
}
