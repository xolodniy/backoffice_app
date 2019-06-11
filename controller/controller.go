package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"net/http"

	"backoffice_app/app"
	"backoffice_app/config"

	"github.com/gin-gonic/gin"
)

// this variables should be specified by '--ldflags' on the building stage
var branch, commit, author, date, summary string

// Controller implements main api object
type Controller struct {
	Config config.Main
	Gin    *gin.Engine
	App    app.App
}

// New returns controller object
func New(config config.Main, app *app.App) *Controller {
	return &Controller{
		Config: config,
		Gin:    gin.Default(),
		App:    *app,
	}
}

// Start starts server
func (c *Controller) Start() {
	c.initRoutes()

	srv := &http.Server{
		Addr:    ":" + c.Config.GinPort,
		Handler: c.Gin,
	}

	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}

func (c *Controller) initRoutes() {
	c.Gin.GET("/healthcheck", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"result": "ok"})
	})
	c.Gin.GET("/api/v1/revision", func(ctx *gin.Context) {
		c.respondOK(ctx, gin.H{
			"branch":  branch,
			"commit":  commit,
			"author":  author,
			"date":    date,
			"summary": summary,
		})
	})

	jira := c.Gin.Group("")
	jira.POST("/api/v1/jira/webhooks/issue/updated", c.issueUpdated)
	jira.POST("/api/v1/jira/webhooks/issue/started", c.issueStarted)

	bitbucket := c.Gin.Group("")
	bitbucket.POST("/api/v1/bitbucket/webhooks/commit/pushed", c.commitPushed)

	slack := c.Gin.Group("")
	slack.Use(c.checkSignature)
	slack.POST("/api/v1/slack/sprintreport", c.sprintReport)
	slack.POST("/api/v1/slack/current_activity", c.slackCurrentActivityHandler)
	slack.POST("/api/v1/slack/customreport", c.customReport)
	slack.POST("/api/v1/slack/afk", c.afkCommand)
	slack.POST("/api/v1/slack/messages/check", c.messagesCheck)
	slack.POST("/api/v1/slack/sprintstatus", c.sprintStatus)
	slack.POST("/api/v1/slack/vacation", c.vacation)
}

func (c *Controller) checkSignature(ctx *gin.Context) {
	body, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{
			"error": "Fail to authorize",
		})
		ctx.Abort()
		return
	}
	ctx.Request.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(body))) //return response body back
	timestamp := ctx.GetHeader("X-Slack-Request-Timestamp")
	signature := ctx.GetHeader("X-Slack-Signature")
	secret := []byte(c.App.Slack.Secret)
	hash := hmac.New(sha256.New, secret)
	hash.Write([]byte("v0:" + timestamp + ":" + string(body)))
	if "v0="+hex.EncodeToString(hash.Sum(nil)) != signature {
		ctx.JSON(http.StatusForbidden, gin.H{
			"error": "Fail to authorize",
		})
		ctx.Abort()
		return
	}
	ctx.Next()
}
