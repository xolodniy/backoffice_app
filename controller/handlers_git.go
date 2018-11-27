package controller

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var r, _ = regexp.Compile(`/((etc|db)/migrations/[0-9]{4,}([0-9a-zA-Z_]+)?\.sql)`)

type File string

type Commit struct {
	Added    []File `json:"added"`
	Modified []File `json:"modified"`
	Removed  []File `json:"removed"`
}

type req struct {
	EventName  string `json:"object_kind" binding:"required"`
	BranchPath string `json:"ref"    binding:"required"`

	Project struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	UserAvatar string `json:"user_avatar"`
	UserName   string `json:"user_name"`

	Commits           []Commit `json:"commits"`
	TotalCommitsCount int      `json:"total_commits_count"`
}

func (c *Controller) gitHandlerOnEventPush(ctx *gin.Context) {

	var req req
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.respondBindingError(ctx, err, req)
		return
	}

	if req.EventName != "push" {
		c.respondError(ctx, fmt.Errorf("Only push event will be accepted."))
		return
	}

	var message string
	if req.TotalCommitsCount > 20 {
		message += "*Warning! Some migration can be skipped which are in commits placed beyond the 20 commit barrier*\n"
	}

	c.App.Slack.SendStandardMessageWithIcon(
		message,
		c.Config.Slack.Channel.BackOfficeAppID,
		req.UserName+" (bot)",
		req.UserAvatar,
	)

	for _, commit := range req.Commits {
		for _, f := range commit.Added {
			message := "ADDED:\n"
			if occurrences := r.FindStringSubmatch(string(f)); len(occurrences) > 0 {
				message += req.Project.Name + ", " + req.BranchPath + ", " + occurrences[0] + ":" + "\n"
				if fileContents, err := c.App.GitGetFile(
					req.Project.ID,
					strings.Replace(occurrences[0], "/", "", 1),
					req.BranchPath,
				); err != nil {
					message += fmt.Sprintf("Error occurred: %v", err)
				} else {
					message += fmt.Sprintf("```%s```", fileContents)
				}

				c.App.Slack.SendStandardMessageWithIcon(
					message,
					c.Config.Slack.Channel.BackOfficeAppID,
					req.UserName+" (bot)",
					req.UserAvatar,
				)
				//fmt.Println(message)
			}
		}
		for _, f := range commit.Modified {
			message := "MODIFIED:\n"
			if occurrences := r.FindStringSubmatch(string(f)); len(occurrences) > 0 {
				message += req.Project.Name + ", " + req.BranchPath + ", " + occurrences[0] + ":" + "\n"
				if fileContents, err := c.App.GitGetFile(
					req.Project.ID,
					strings.Replace(occurrences[0], "/", "", 1),
					req.BranchPath,
				); err != nil {
					message += fmt.Sprintf("Error occurred: %v", err)
				} else {
					message += fmt.Sprintf("```%s```", fileContents)
				}
				c.App.Slack.SendStandardMessageWithIcon(
					message,
					c.Config.Slack.Channel.BackOfficeAppID,
					req.UserName+" (bot)",
					req.UserAvatar,
				)
			}
		}
	}

	c.respondOK(ctx, "ok")
}
