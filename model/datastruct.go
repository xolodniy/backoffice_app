package model

import (
	"fmt"
)

var (
	// ErrorInternal returns to controller if request valid but domain can't do it
	ErrorInternal = fmt.Errorf("ошибка сервера, обратитесь в службу поддержки или повторите попытку позже")
	// ErrNotFound if record does not found
	ErrNotFound = fmt.Errorf("запись не найдена")
)
