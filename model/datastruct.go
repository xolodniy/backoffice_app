package model

import (
	"fmt"
)

var (
	// InternalError returns to controller if request valid but domain can't do it
	InternalError = fmt.Errorf("ошибка сервера, обратитесь в службу поддержки или повторите попытку позже")
	// NotFoundError if record does not found
	NotFoundError = fmt.Errorf("запись не найдена")
)
