// All direct slack commands handling in this place
// TODO: move old commands from app.go
package app

import (
	"fmt"

	"backoffice_app/common"
	"backoffice_app/model"
)

// Protect branch or pool request for prevent show it in report
func (a *App) Protect(userID, name, comment string) error {
	var b model.Protected
	err := a.model.First(&b, model.Protected{Name: name})
	if err == common.ErrInternal {
		return err
	}
	if err == nil {
		userName, ok := a.Config.GetUserInfoByTagValue(TagUserSlackID, userID)[TagUserSlackRealName]
		if !ok {
			userName = userID // prevent situation when user info not found in configuration
		}
		return fmt.Errorf("branch '%s' already protected by %s with comment '%s'",
			b.Name, userName, b.Comment)
	}

	return a.model.Create(&model.Protected{
		Name:    name,
		Comment: comment,
		UserID:  userID,
	})
}

func (a *App) Unprotect(userID, branchName string) error {
	return a.model.Delete(&model.Protected{Name: branchName})
}

func (a *App) ShowProtected(channel string) {
	var branches []model.Protected
	if err := a.model.Find(&branches); err != nil {
		return
	}
	if len(branches) == 0 {
		a.Slack.SendMessage("Protected branches not found", channel)
		return
	}
	var message string
	for _, b := range branches {
		userName, ok := a.Config.GetUserInfoByTagValue(TagUserSlackID, b.UserID)[TagUserSlackRealName]
		if !ok {
			userName = b.UserID // prevent situation when user info not found in configuration
		}
		message += fmt.Sprintf("%50s %30s %s\n", b.Name, userName, b.Comment)
	}
	a.Slack.SendMessage(message, channel)
}
