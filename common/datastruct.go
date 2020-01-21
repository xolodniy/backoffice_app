package common

import "errors"

const (
	CommitTypeMigration = "migration"
	CommitTypeAnsible   = "ansible"

	DevTeamBackend  = "backend"
	DevTeamFrontend = "frontend"

	OnDutyBe = "ondutybe"
	OnDutyFe = "ondutyfe"

	FETeam = "feteam"
	BETeam = "beteam"
	QATeam = "qateam"
)

// ErrConflict implements error for HTTP 409
type ErrConflict struct {
	Msg string
}

// Error implements error interface
func (err ErrConflict) Error() string {
	if err.Msg == "" {
		return "Ошибка состояния данных"
	}
	return err.Msg
}

// ErrNotFound implements errpr for HTTP 404
type ErrNotFound struct {
	Msg string
}

// Error implements error interface
func (err ErrNotFound) Error() string {
	if err.Msg == "" {
		return "Запись не найдена"
	}
	return err.Msg
}

var (
	ErrInternal      = errors.New("Внутренняя ошибка сервера, повторите попытку позже или обратитесь к системному администратору")
	ErrModelNotFound = errors.New("Запись не найдена")
)
