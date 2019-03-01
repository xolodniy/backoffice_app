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

// Controller implements main api object
type Controller struct {
	Config config.Main
	Gin    *gin.Engine
	App    app.App
}

// New returns controller object
func New(config config.Main) *Controller {
	appObj := app.New(&config)
	if !config.GinDebugMode {
		gin.SetMode(gin.ReleaseMode)
	}
	return &Controller{
		Config: config,
		Gin:    gin.Default(),
		App:    *appObj,
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

	jira := c.Gin.Group("")
	jira.POST("/rest/jira/webhooks/issue/created", c.issueCreated)

	bitbucket := c.Gin.Group("")
	bitbucket.POST("/rest/bitbucket/webhooks/commit/pushed", c.commitPushed)

	slack := c.Gin.Group("")
	slack.Use(c.checkSignature)
	slack.POST("/api/v1/slack/sprintreport", c.sprintReport)
	slack.POST("/api/v1/slack/last_activity", c.slackLastActivityHandler)
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
