package common

import "fmt"

const (
	CommitTypeMigration = "migration"
	CommitTypeAnsible   = "ansible"

	DevTeamBackend  = "backend"
	DevTeamFrontend = "frontend"

	OnDutyBe = "ondutybe"
	OnDutyFe = "ondutyfe"
)

var (
	ErrInternal = fmt.Errorf("Внутренняя ошибка сервера, повторите попытку позже или обратитесь к системному администратору")
	ErrNotFound = fmt.Errorf("Запись не найдена")
)
