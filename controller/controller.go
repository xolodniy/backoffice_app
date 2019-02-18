package controller

import (
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

	c.Gin.POST("/api/v1/slack/last_activity", c.slackLastActivityHandler)
}
