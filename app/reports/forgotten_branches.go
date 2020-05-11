// report about branches with no activity
// does request with all branches to bitbucket and compares they with already stored in DB
// branches which stored in DB so long will be showed in report
// some protected branches will be omitted
// you can protect branch by command /skip in slack
package reports

import (
	"regexp"
	"time"

	"backoffice_app/common"
	"backoffice_app/config"
	"backoffice_app/model"
	"backoffice_app/services/bitbucket"
	"backoffice_app/services/slack"

	"github.com/sirupsen/logrus"
)

type ForgottenBranches struct {
	bitbucket bitbucket.Bitbucket
	config    config.Main
	model     model.Model
	slack     slack.Slack
}

func NewReportForgottenBranches(
	m model.Model,
	b bitbucket.Bitbucket,
	c config.Main,
	s slack.Slack,
) ForgottenBranches {
	return ForgottenBranches{
		bitbucket: b,
		config:    c,
		model:     m,
		slack:     s,
	}
}

// Run checks branches without pull requests
func (fb ForgottenBranches) Run() {
	forgottenBranches, err := fb.model.GetForgottenBranches()
	if err != nil {
		return
	}
	branchesWithoutPRs, err := fb.bitbucket.BranchesWithoutPullRequests()
	if err != nil {
		return
	}
	protected, err := fb.model.GetNamesOfProtectedBranchesAndPRs()
	if err != nil {
		return
	}
	r, err := regexp.Compile("^(release|hotfix)/[0-9]{8}")
	if err != nil {
		logrus.WithError(err).WithField("regexp", "^(release|hotfix)/[0-9]{8}").Error("Can't compile regexp")
		return
	}
	m := make(map[string][]string)
	for _, branch := range branchesWithoutPRs {
		if common.ValueIn(branch.Name, protected...) || r.MatchString(branch.Name) || branch.Name == "master" {
			continue
		}
		var exists bool
		for i := range forgottenBranches {
			if branch.Name != forgottenBranches[i].Name || branch.Target.Repository.Name != forgottenBranches[i].RepoSlug {
				continue
			}
			if forgottenBranches[i].CreatedAt.Before(time.Now().AddDate(0, -1, 0)) {
				user := branch.Target.Author.User.DisplayName
				m[user] = append(m[user], "<"+branch.Links.HTML.Href+"|"+branch.Name+">")
				exists = true
				break
			}
		}
		if !exists {
			fb.model.Create(&model.ForgottenBranch{
				Name:     branch.Name,
				RepoSlug: branch.Target.Repository.Name,
			})
		}
	}
	if len(m) == 0 {
		return
	}

	message := "*Обнаружены брошенные ветки*\n"
	for author, branches := range m {
		slackID, ok := fb.config.GetUserInfoByTagValue("slackrealname", author)["slackid"]
		if !ok {
			slackID = author
		} else {
			slackID = "<@" + slackID + ">"
		}
		for i := range branches {
			message += "\n" + branches[i]
		}
		message += "\n" + slackID + "\n"
	}
	fb.slack.SendMessage(message, "#back-office-app")
}

// Run checks branches without pull requests
//func (fb ForgottenBranches) Run(channel string) {
//	forgottenBranches, err := fb.model.GetForgottenBranches()
//	if err != nil {
//		return
//	}
//	branchesWithoutPRs, err := fb.bitbucket.BranchesWithoutPullRequests()
//	if err != nil {
//		return
//	}
//	protected, err := fb.model.GetNamesOfProtectedBranchesAndPRs()
//	if err != nil {
//		return
//	}
//	protected = append(protected, "master", "dev")
//
//	var (
//		firstAttentionBranches  = make(map[string][]string)
//		secondAttentionBranches = make(map[string][]string)
//		thirdAttentionBranches  = make(map[string][]string)
//	)
//	r, err := regexp.Compile("^(release|hotfix)/[0-9]{8}")
//	if err != nil {
//		logrus.WithError(err).WithField("regexp", "^(release|hotfix)/[0-9]{8}").Error("Can't compile regexp")
//		return
//	}
//	for _, branch := range branchesWithoutPRs {
//		if common.ValueIn(branch.Name, protected...) || r.Match([]byte(branch.Name)) {
//			continue
//		}
//
//		// TODO: move slackrealname & slackid to constant
//		userSlackMention := "<@" + fb.config.GetUserInfoByTagValue("slackrealname", branch.Target.Author.User.DisplayName)["slackid"] + ">"
//		if fb.config.GetUserInfoByTagValue("slackrealname", branch.Target.Author.User.DisplayName)["slackid"] == "" {
//			userSlackMention = "Имя пользователя не удалось определить"
//		}
//		var isExist bool
//		for i := len(forgottenBranches) - 1; i >= 0; i-- {
//			if branch.Name != forgottenBranches[i].Name || branch.Target.Repository.Name != forgottenBranches[i].RepoSlug {
//				continue
//			}
//			switch {
//			case forgottenBranches[i].CreatedAt.Before(time.Now().AddDate(0, 0, -7)):
//				if err := fb.model.DeleteForgottenBranch(forgottenBranches[i].Name, forgottenBranches[i].RepoSlug); err != nil {
//					return
//				}
//				// TODO: remove third attention, add deleting branches
//				thirdAttentionBranches[userSlackMention] = append(thirdAttentionBranches[userSlackMention], "<"+branch.Links.HTML.Href+"|"+branch.Name+">")
//				//a.Bitbucket.DeleteBranch(branch.RepoSlug, branch.Name)
//			case forgottenBranches[i].CreatedAt.Before(time.Now().AddDate(0, 0, -6)):
//				secondAttentionBranches[userSlackMention] = append(secondAttentionBranches[userSlackMention], "<"+branch.Links.HTML.Href+"|"+branch.Name+">")
//			}
//			forgottenBranches[i] = forgottenBranches[len(forgottenBranches)-1]
//			forgottenBranches = forgottenBranches[:len(forgottenBranches)-1]
//			isExist = true
//			break
//		}
//		if !isExist {
//			firstAttentionBranches[userSlackMention] = append(firstAttentionBranches[userSlackMention], "<"+branch.Links.HTML.Href+"|"+branch.Name+">")
//			if err := fb.model.CreateForgottenBranches(model.ForgottenBranch{
//				RepoSlug: branch.Target.Repository.Name,
//				Name:     branch.Name,
//			}); err != nil {
//				return
//			}
//		}
//	}
//	for _, b := range forgottenBranches {
//		if err := fb.model.DeleteForgottenBranch(b.Name, b.RepoSlug); err != nil {
//			return
//		}
//	}
//	for author, prLinks := range firstAttentionBranches {
//		firstAttention := "\n" + author + "\n"
//		for _, link := range prLinks {
//			firstAttention += link + "\n"
//		}
//		fb.slack.SendMessage("Если для этих веток в течение 7 дней не будут созданы PR, они идут нахер:\n"+firstAttention, channel)
//	}
//	for author, prLinks := range secondAttentionBranches {
//		secondAttention := "\n" + author + "\n"
//		for _, link := range prLinks {
//			secondAttention += link + "\n"
//		}
//		fb.slack.SendMessage("Если в этих ветках в течение дня не будут созданы PR, они будут удалены:\n"+secondAttention, channel)
//	}
//	// TODO: remove third attention
//	for author, prLinks := range thirdAttentionBranches {
//		thirdAttention := "\n" + author + "\n"
//		for _, link := range prLinks {
//			thirdAttention += link + "\n"
//		}
//		fb.slack.SendMessage("Удалены(фактически нет):\n"+thirdAttention, channel)
//	}
//}
