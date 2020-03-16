// All direct slack commands handling in this place
// TODO: move old commands from app.go
package app

import (
	"fmt"
	"time"

	"backoffice_app/model"
)

func (a *App) ProtectBranch(userID, branchName, comment string) error {
	return a.model.Save(&model.ProtectedBranch{
		Name:    branchName,
		Comment: comment,
		UserID:  a.Config.GetUserInfoByTagValue(TagUserSlackID, userID)[TagUserSlackRealName],
	})
}

func (a *App) UnprotectBranch(userID, branchName string) error {
	now := time.Now()
	return a.model.Save(&model.ProtectedBranch{
		Name:      branchName,
		UserID:    a.Config.GetUserInfoByTagValue(TagUserSlackID, userID)[TagUserSlackRealName],
		DeletedAt: &now,
	})
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
		message += fmt.Sprintf("%30s %20s %s\n", b.Name, b.UserID, b.Comment)
	}
	a.Slack.SendMessage(message, channel)
}
