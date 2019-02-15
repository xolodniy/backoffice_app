package controller

import (
	"net/http"

	"backoffice_app/types"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func (c *Controller) slackLastActivityHandler(ctx *gin.Context) {

	for i, p := range ctx.Params {
		logrus.Warn("header", i, p.Key, p.Value)
	}

	logrus.Warn("Conttype", ctx.GetHeader("Content-type"))

	buf := make([]byte, 2048)
	_, err := ctx.Request.Body.Read(buf)
	if err != nil {
		logrus.WithError(err).Error("cannot read the body")
	}
	logrus.Warn("Body", string(buf))

	ctx.JSON(http.StatusOK, struct {
		text        string             `json:"text"`
		attachments []types.Attachment `json:"attachments"`
	}{
		text: "just response text", attachments: []types.Attachment{{Text: "text1"}, {"text2"}},
	})

	//

}
