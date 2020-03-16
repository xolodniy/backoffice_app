// All direct slack commands handling in this place
// TODO: move old commands from app.go
package app

import (
	"fmt"

	"backoffice_app/common"
	"backoffice_app/model"
)

func (a *App) ProtectBranch(userID, branchName, comment string) error {
	var b model.ProtectedBranch
	err := a.model.First(&b, model.ProtectedBranch{Name: branchName})
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

	return a.model.Create(&model.ProtectedBranch{
		Name:    branchName,
		Comment: comment,
		UserID:  userID,
	})
}

func (a *App) UnprotectBranch(userID, branchName string) error {
	return a.model.Delete(&model.ProtectedBranch{Name: branchName})
}

func (a *App) ShowProtectedBranches(channel string) {
	var branches []model.ProtectedBranch
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
