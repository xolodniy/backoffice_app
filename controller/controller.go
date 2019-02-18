package controller

import (
	"bytes"
	"github.com/sirupsen/logrus"
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
	c.Gin.POST("/sprintreport", c.sprintReport)
}

func (c *Controller) sprintReport(ctx *gin.Context) {
	body, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		logrus.Debug(err)
	}
	ctx.Request.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(body))) //return response body back
	timestamp := ctx.GetHeader("X-Slack-Request-Timestamp")
	signature := ctx.GetHeader("X-Slack-Signature")
	userId := ctx.PostForm("user_id")
	text := ctx.PostForm("text")
	if c.App.CheckSignature(signature, []byte("v0:"+timestamp+":"+string(body))) {
		go func() {
			err := c.App.ReportSprintsIsuues(text, userId)
			if err != nil {
				c.App.Slack.SendMessage(err.Error(), userId, true)
			}
		}()
		ctx.JSON(http.StatusOK, "ok, wait for report")
		return
	}
	ctx.JSON(http.StatusOK, "Fail to authorize")
}
