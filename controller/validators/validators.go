package validators

import (
	"fmt"

	"gopkg.in/go-playground/validator.v8"
)

// GetErrorResponse returns specific field error
func GetErrorResponse(e *validator.FieldError) string {
	switch e.ActualTag {
	default:
		return fmt.Sprintf("Not matches '%s'", e.ActualTag)
	}
}
