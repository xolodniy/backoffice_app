// report about pull requests with no activity
// does request with all pull requests to bitbucket and compares they with already stored in DB
// pull requests which stored in DB so long will be showed in report
// some protected pull requests will be omitted
// you can protect pull request by command /skip in slack
package reports

import (
	"time"

	"backoffice_app/common"
	"backoffice_app/config"
	"backoffice_app/model"
	"backoffice_app/services/bitbucket"
	"backoffice_app/services/slack"
)

type ForgottenPullRequests struct {
	bitbucket bitbucket.Bitbucket
	config    config.Main
	model     model.Model
	slack     slack.Slack
}

func NewReportForgottenPullRequests(
	m model.Model,
	b bitbucket.Bitbucket,
	c config.Main,
	s slack.Slack,
) ForgottenPullRequests {
	return ForgottenPullRequests{
		bitbucket: b,
		config:    c,
		model:     m,
		slack:     s,
	}
}

// CheckForgottenGitPullRequests checks pull requests on activity
func (fpr ForgottenPullRequests) Run(channel string) {
	forgottenPullRequests, err := fpr.model.GetForgottenPullRequest()
	if err != nil {
		return
	}
	pullRequests, err := fpr.bitbucket.PullRequestsActivity()
	if err != nil {
		return
	}
	protected, err := fpr.model.GetNamesOfProtectedBranchesAndPRs()
	if err != nil {
		return
	}
	var (
		firstAttentionPRs  = make(map[string][]string)
		secondAttentionPRs = make(map[string][]string)
		thirdAttentionPRs  = make(map[string][]string)
	)
	for _, pr := range pullRequests {
		if common.ValueIn(pr.Title, protected...) {
			continue
		}
		userSlackMention := "<@" + fpr.config.GetUserInfoByTagValue("slackrealname", pr.Author.DisplayName)["slackid"] + ">"
		if fpr.config.GetUserInfoByTagValue("slackrealname", pr.Author.DisplayName)["slackid"] == "" {
			userSlackMention = "Имя пользователя не удалось определить"
		}
		lastActivity := pr.LastActivityDate()
		// if this pull request without activity last 5 days, it is old and we create it in database
		if lastActivity.After(time.Now().AddDate(0, 0, -5)) {
			continue
		}
		var isExist bool
		for i := len(forgottenPullRequests) - 1; i >= 0; i-- {
			if forgottenPullRequests[i].PullRequestID != pr.ID || forgottenPullRequests[i].RepoSlug != pr.Source.Repository.Name {
				continue
			}
			switch {
			case lastActivity.Before(time.Now().AddDate(0, 0, -8)) && forgottenPullRequests[i].CreatedAt.Before(time.Now().AddDate(0, 0, -3)):
				if err := fpr.model.DeleteForgottenPullRequest(forgottenPullRequests[i].PullRequestID, forgottenPullRequests[i].RepoSlug); err != nil {
					return
				}
				// TODO: remove third attention, add declining PRs
				thirdAttentionPRs[userSlackMention] = append(thirdAttentionPRs[userSlackMention], "<"+pr.Links.HTML.Href+"|"+pr.Title+">")
				//a.Bitbucket.DeclinePullRequest(pr.RepoSlug, pr.PullRequestID)
			case lastActivity.Before(time.Now().AddDate(0, 0, -7)) && forgottenPullRequests[i].CreatedAt.Before(time.Now().AddDate(0, 0, -2)):
				secondAttentionPRs[userSlackMention] = append(secondAttentionPRs[userSlackMention], "<"+pr.Links.HTML.Href+"|"+pr.Title+">")
			}
			forgottenPullRequests[i] = forgottenPullRequests[len(forgottenPullRequests)-1]
			forgottenPullRequests = forgottenPullRequests[:len(forgottenPullRequests)-1]
			isExist = true
			break
		}
		if !isExist {
			firstAttentionPRs[userSlackMention] = append(firstAttentionPRs[userSlackMention], "<"+pr.Links.HTML.Href+"|"+pr.Title+">")
			if err := fpr.model.CreateForgottenPullRequest(model.ForgottenPullRequest{
				PullRequestID: pr.ID,
				RepoSlug:      pr.Source.Repository.Name,
			}); err != nil {
				return
			}

		}
	}
	for _, pr := range forgottenPullRequests {
		if err := fpr.model.DeleteForgottenPullRequest(pr.PullRequestID, pr.RepoSlug); err != nil {
			return
		}
	}
	for author, prLinks := range firstAttentionPRs {
		firstAttention := "\n" + author + "\n"
		for _, link := range prLinks {
			firstAttention += link + "\n"
		}
		fpr.slack.SendMessage("В этих ПР давно нет активности, необходимо это исправить:\n"+firstAttention, channel)
	}
	for author, prLinks := range secondAttentionPRs {
		secondAttention := "\n" + author + "\n"
		for _, link := range prLinks {
			secondAttention += link + "\n"
		}
		fpr.slack.SendMessage("Если в этих ПР в течение дня не будет никакой активности, они идут нахер:\n"+secondAttention, channel)
	}
	// TODO: remove third attention
	for author, prLinks := range thirdAttentionPRs {
		thirdAttention := "\n" + author + "\n"
		for _, link := range prLinks {
			thirdAttention += link + "\n"
		}
		fpr.slack.SendMessage("Удалены(фактически нет):\n"+thirdAttention, channel)
	}
}
