package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"time"

	"backoffice_app/app"
	"backoffice_app/config"

	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
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

	// external middleware for repeating http requests to sentry.io. will be used when sentry enabled in config only
	if c.Config.Sentry.EnableSentry != nil && *c.Config.Sentry.EnableSentry &&
		c.Config.Sentry.LoggingHTTPRequests != nil && *c.Config.Sentry.LoggingHTTPRequests {
		c.Gin.Use(SendHTTPLogsToSentry())
	}

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
	jira.POST("/api/v1/jira/webhooks/issue/comment/created", c.issueCommentCreated)

	bitbucket := c.Gin.Group("")
	bitbucket.POST("/api/v1/bitbucket/webhooks/commit/pushed", c.commitPushed)
	bitbucket.POST("/api/v1/bitbucket/webhooks/pullrequest/merged", c.pullRequestMerged)

	slack := c.Gin.Group("")
	slack.Use(c.checkSignature)
	slack.POST("/api/v1/slack/sprintreport", c.sprintReport)
	slack.POST("/api/v1/slack/current_activity", c.slackCurrentActivityHandler)
	slack.POST("/api/v1/slack/customreport", c.customReport)
	slack.POST("/api/v1/slack/afk", c.afkCommand)
	slack.POST("/api/v1/slack/messages/check", c.messagesCheck)
	slack.POST("/api/v1/slack/sprintstatus", c.sprintStatus)
	slack.POST("/api/v1/slack/vacation", c.vacation)
	slack.POST("/api/v1/slack/set-onduty-be", c.setOnDutyBackend)
	slack.POST("/api/v1/slack/set-onduty-fe", c.setOnDutyFrontend)
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

// custom writer enables logs http response
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write default method for the Writer interface
func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// SendHTTPLogsToSentry middleware for sending http request and response to sentry.io
func SendHTTPLogsToSentry() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		req, err := httputil.DumpRequest(ctx.Request, true)
		if err != nil {
			logrus.WithError(err).Error("can't dump http request for sentry")
		}

		// replaced gin writer allows duplicate info for logs
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: ctx.Writer}
		ctx.Writer = blw

		defer func() {
			// we dont want to log 2xx, 3xx, 404 codes into Sentry, especially coz we fall during upload big ZIP bodies
			if ctx.Writer.Status() < 400 || ctx.Writer.Status() == 404 {
				return
			}
			level := raven.DEBUG
			if ctx.Writer.Status() >= 400 {
				level = raven.WARNING
			}
			if ctx.Writer.Status() >= 500 {
				level = raven.ERROR
			}

			extra := raven.Extra{}
			for i, item := range ctx.Errors {
				level = raven.ERROR
				extra[fmt.Sprint("ginError ", i+1)] = item.Error()
			}
			if rval := recover(); rval != nil {
				level = raven.FATAL
				extra["recovered"] = fmt.Sprint(rval)
				ctx.AbortWithStatus(http.StatusInternalServerError)
			}

			response := blw.body.Bytes()
			// optional try to make pretty output (if response is json object)
			var i interface{}
			if err := json.Unmarshal(response, &i); err == nil {
				response, err = json.MarshalIndent(i, "", "	")
				if err != nil {
					logrus.WithError(err).Error("can't dump http response for sentry")
				}
			}
			extra["request"] = string(req)
			extra["response"] = string(response)
			raven.DefaultClient.Capture(&raven.Packet{
				Message:   ctx.Request.URL.Path, // TODO change url to template after https://github.com/gin-gonic/gin/pull/1826 released
				Project:   "CDTO",
				Timestamp: raven.Timestamp(time.Now()),
				Level:     level,
				Extra:     extra,
			}, map[string]string{
				"endpoint": ctx.Request.URL.Path,
			})
		}()

		ctx.Next()
	}
}
