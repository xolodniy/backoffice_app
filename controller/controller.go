package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"github.com/gin-gonic/gin/binding"
	"io/ioutil"
	"net/http"

	"backoffice_app/app"
	"backoffice_app/config"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Controller implements main api object
type Controller struct {
	Config config.Main
	Gin    *gin.Engine
	App    app.App
}

// Slack struct of slack slach commands request https://api.slack.com/slash-commands
type SlackReq struct {
	Token       string `form:"token" binding:"required"`
	TeamId      string `form:"team_id" binding:"required"`
	TeamDomain  string `form:"team_domain" binding:"required"`
	ChannelId   string `form:"channel_id" binding:"required"`
	ChannelName string `form:"channel_name" binding:"required"`
	UserId      string `form:"user_id" binding:"required"`
	Text        string `form:"text" binding:"required"`
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
	slack := c.Gin.Group("")
	slack.Use(c.checkSignature)
	slack.POST("/sprintreport", c.sprintReport)
	c.Gin.POST("/api/v1/slack/last_activity", c.slackLastActivityHandler)
}

func (c *Controller) sprintReport(ctx *gin.Context) {
	request := SlackReq{}
	err := ctx.ShouldBindWith(&request, binding.FormPost)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	if request.UserId != "" && request.Text != "" {
		go func() {
			err := c.App.ReportSprintsIsuues(request.Text, request.UserId)
			if err != nil {
				c.App.Slack.SendMessage(err.Error(), request.UserId, true)
			}
		}()
		ctx.JSON(http.StatusOK, "ok, wait for report")
		return
	}
	ctx.JSON(http.StatusOK, "Empty variables")
}

func (c *Controller) checkSignature(ctx *gin.Context) {
	body, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		logrus.Debug(err)
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
