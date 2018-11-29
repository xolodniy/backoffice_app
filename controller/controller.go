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
	appObj, err := app.New(&config)
	if err != nil {
		panic(err)
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
	srv.ListenAndServe()
}

func (c *Controller) initRoutes() {
	c.Gin.GET("/healthcheck", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"result": "ok"})
	})

	// this endpoint will be used to receive data about Push actions made on GitLab on pushing local changes to remote branch on GitLab repository
	c.Gin.POST("/api/v1/git/onevent/push", c.gitHandlerOnEventPush)

}
