package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"backoffice_app/controller/validators"
	"backoffice_app/model"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/go-playground/validator.v8"
)

// FieldErrors use it for returning field validation errors on manual validations
type FieldErrors map[string]string

// Error mandatory method of error interface
func (f FieldErrors) Error() string {
	var str string
	for key, message := range f {
		str += key + ":" + message + "\n"
	}
	return str
}

func (c *Controller) respondError(ctx *gin.Context, err error) {
	if fieldErrors, ok := err.(FieldErrors); ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"fieldErrors": fieldErrors})
		return
	}

	h := gin.H{"error": err.Error()}
	if err == model.InternalError {
		ctx.JSON(http.StatusInternalServerError, h)
	} else if err == model.NotFoundError {
		ctx.JSON(http.StatusNotFound, h)
	} else {
		ctx.JSON(http.StatusBadRequest, h)
	}
}

func (c *Controller) respondBindingError(ctx *gin.Context, err error, req interface{}) {
	switch v := err.(type) {
	case validator.ValidationErrors:
		fieldErrors := make(FieldErrors)
		for _, e := range v {
			structField, ok := reflect.TypeOf(req).FieldByName(e.Field)
			if !ok {
				logrus.WithFields(logrus.Fields{
					"expectedField":  e.Field,
					"receivedStruct": reflect.TypeOf(req),
				}).Error("expected field is missing")
				continue
			}
			fieldErrors[structField.Tag.Get("json")] = validators.GetErrorResponse(e)
		}
		c.respondError(ctx, fieldErrors)
	case *json.SyntaxError, *json.UnmarshalTypeError:
		c.respondError(ctx, fmt.Errorf("тело запроса должно быть в формате JSON"))
	default:
		c.respondError(ctx, err)
	}
}

func (c *Controller) respondOK(ctx *gin.Context, result interface{}) {
	ctx.JSON(http.StatusOK, gin.H{"result": result})
}
