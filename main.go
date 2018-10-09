package main

import (
	"backoffice_app/services/hubstaff"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/andygrunwald/go-jira"

	"backoffice_app/config"
	"backoffice_app/types"
)

var HSAuthToken = ""
var HSAppToken = "yWDG5mMG3yln_GaIg-P5vnvlKlWeXZC9IE9cqAuDkoQ"
var HSLogin = ""
var HSPassword = ""
var HSOursOrgsID int64 = 60470

var SlackOutToken = ""
var SlackChannelID = "#leads-bot-development"
var SlackBotName = "Back Office Bot"

var dateOfWorkdaysStart = time.Date(2018, 9, 10, 0, 0, 0, 0, time.Local)
var dateOfWorkdaysEnd = time.Date(2018, 9, 11, 23, 59, 59, 0, time.Local)

func main() {

	cfg := config.GetConfig()
	HSOursOrgsID = cfg.HubStaff.OrgsID

	SlackOutToken = cfg.Slack.Auth.OutToken
	SlackChannelID = "#" + cfg.Slack.Channel.ID
	SlackBotName = cfg.Slack.Channel.BotName

	jiraClient, err := jiraMakeOAuth(cfg.Jira.Auth)
	if err != nil {
		panic(err)
	}
	jiraAllIssues, _, err := jiraIssuesSearch(cfg.Jira.IssueSearchParams, jiraClient)
	if err != nil {
		panic(err)
	}

	fmt.Printf("jiraAllIssues quantity: %v\n", len(jiraAllIssues))

	HubStaff, err := hubstaff.New(cfg.HubStaff)
	if err != nil {
		panic(fmt.Sprintf("HubStaff error %v: ", err))
	}

	orgsList, err := HubStaff.GetWorkersTimeByOrganization(dateOfWorkdaysStart, dateOfWorkdaysEnd, cfg.HubStaff.OrgsID)
	if err != nil {
		panic(fmt.Sprintf("HubStaff error: %v", err))
	}

	if len(orgsList) == 0 {
		err := sendStandardMessage("No tracked time for now or no organization found")
		if err != nil {
			panic(fmt.Sprintf("Slack error: can't decode response: %s", err))
		}
	}

	fmt.Printf("\nHubStaff output: %v\n", orgsList)

	var message = fmt.Sprintf(
		"Work time report\n\nFrom: %v %v\nTo: %v %v\n",
		dateOfWorkdaysStart.Format("02.01.06"), "00:00:00",
		dateOfWorkdaysEnd.Format("02.01.06"), "23:59:59",
	)
	for _, worker := range orgsList[0].Workers {
		message += fmt.Sprintf(
			"\n%s %s",
			secondsToClockTime(worker.TimeWorked),
			worker.Name,
		)
	}

	if len(orgsList[0].Workers) == 0 {
		message = "No tracked time for now or no workers found"
	}

	if err := sendStandardMessage(message); err != nil {
		panic(err)
	}
}

func jiraMakeOAuth(jba jira.BasicAuthTransport) (*jira.Client, error) {

	client, err := jira.NewClient(jba.Client(), "https://theflow.atlassian.net")
	if err != nil {
		return nil, fmt.Errorf("can't create jira client: %s", err)
	}

	return client, err
}

func jiraIssuesSearch(searchParams config.JiraIssueSearchParams, client *jira.Client) ([]jira.Issue, *jira.Response, error) {
	// allIssues including issues from other sprints and not closed
	var _, _ = searchParams.JQL, &searchParams.Options
	allIssues, response, err := client.Issue.Search(
		searchParams.JQL,
		searchParams.Options,
	)

	if err != nil {
		return nil, response, fmt.Errorf("can't create jira client: %s", err)
	}

	return allIssues, response, nil
}

func secondsToClockTime(durationInSeconds int) string {
	workTime := time.Second * time.Duration(durationInSeconds)

	Hours := int(workTime.Hours())
	Minutes := int(workTime.Minutes())

	return fmt.Sprintf("%d%d:%d%d", Hours/10, Hours%10, Minutes/10, Minutes%10)

}

func postJSONMessage(jsonData []byte) (string, error) {
	var url = "https://slack.com/api/chat.postMessage"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", SlackOutToken))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	//fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	//fmt.Println("response Body:", string(body))

	return string(body), nil
}
func sendPOSTMessage(message *types.PostChannelMessage) (string, error) {

	b, err := json.Marshal(message)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return "", err
	}

	fmt.Printf("JSON IS %+v:\n", string(b))

	resp, err := postJSONMessage(b)

	return resp, err
}
func postChannelMessage(text string, channelID string, asUser bool, username string) (string, error) {
	var msg = &types.PostChannelMessage{
		Token:    SlackOutToken,
		Channel:  channelID,
		AsUser:   asUser,
		Text:     text,
		Username: username,
	}

	return sendPOSTMessage(msg)
}

//Temporarily added. Will be deleted after basic development stage will be finished.
func sendConsoleMessage(message string) error {
	fmt.Println(
		message,
	)
	return nil
}
func sendStandardMessage(message string) error {
	_, err := postChannelMessage(
		message,
		SlackChannelID,
		false,
		SlackBotName,
	)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return err
	}
	return nil
}
