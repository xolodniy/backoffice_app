package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"backoffice_app/types"
)

var HSAuthToken = ""
var HSAppToken = "yWDG5mMG3yln_GaIg-P5vnvlKlWeXZC9IE9cqAuDkoQ"
var HSLogin = ""
var HSPassword = ""
var HSOursOrgsID = 60470

var SlackOutToken = ""
var SlackChannelID = "#leads-bot-development"
var SlackBotName = "Back Office Bot"

var dateOfWorkdaysStart = time.Date(2018, 9, 10, 0, 0, 0, 0, time.Local)
var dateOfWorkdaysEnd = time.Date(2018, 9, 11, 23, 59, 59, 0, time.Local)

type Client struct {
	// HSAppToken created at https://developer.hubstaff.com/my_apps
	AppToken string

	// (optional) HSAuthToken, previously obtained through ObtainAuthToken
	AuthToken string

	// HTTPClient is required to be passed. Pass http.DefaultClient if not sure
	HTTPClient *http.Client
}

func main() {
	hubstaff := Client{
		AppToken:   HSAppToken,
		AuthToken:  HSAuthToken, // Set it if already known. If not, see below how to obtain it.
		HTTPClient: http.DefaultClient,
	}

	if HSAuthToken == "" {
		var err error
		HSAuthToken, err = hubstaff.ObtainAuthToken(HSLogin, HSPassword)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Your «HSAuthToken» is:\n%v\n", HSAuthToken)
		return
	}

	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")
	orgsRaw, err := hubstaff.doRequest(fmt.Sprintf(
		"/v1/custom/by_member/team/?start_date=%s&end_date=%s&organizations=%d",
		dateStart,
		dateEnd,
		HSOursOrgsID), nil)
	if err != nil {
		panic(err)
	}

	orgs := struct {
		List types.Organizations `json:"organizations"`
	}{}
	err = json.Unmarshal(orgsRaw, &orgs)
	if err != nil {
		panic(fmt.Sprintf("can't decode response: %s", err))
	}

	if len(orgs.List) == 0 {
		err := sendStandardMessage("No tracked time for now or no organization found")
		if err != nil {
			panic(fmt.Sprintf("can't decode response: %s", err))
		}
	}

	var message = fmt.Sprintf(
		"Work time report\n\nFrom: %v %v\nTo: %v %v\n",
		dateOfWorkdaysStart.Format("02.01.06"), "00:00:00",
		dateOfWorkdaysEnd.Format("02.01.06"), "23:59:59",
	)
	for _, worker := range orgs.List[0].Workers {
		message += fmt.Sprintf(
			"\n%s %s",
			secondsToClockTime(worker.TimeWorked),
			worker.Name,
		)
	}

	if len(orgs.List[0].Workers) == 0 {
		message = "No tracked time for now or no workers found"
	}

	if err := sendStandardMessage(message); err != nil {
		panic(err)
	}
}

func secondsToClockTime(durationInSeconds int) string {
	workTime := time.Second * time.Duration(durationInSeconds)

	Hours := int(workTime.Hours())
	Minutes := int(workTime.Minutes())

	return fmt.Sprintf("%d%d:%d%d", Hours/10, Hours%10, Minutes/10, Minutes%10)

}

// Retrieves auth token which must be sent along with appToken,
// see https://support.hubstaff.com/time-tracking-api/ for details
func (c *Client) ObtainAuthToken(email, password string) (string, error) {
	form := url.Values{}
	form.Add("email", email)
	form.Add("password", password)

	r, err := http.NewRequest("POST", "https://api.hubstaff.com/v1/auth", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("can't create http request: %s", err)
	}
	r.Header.Set("App-Token", HSAppToken)
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTPClient.Do(r)
	if err != nil {
		return "", fmt.Errorf("can't send http request: %s", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("invalid response code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	t := struct {
		User struct {
			ID           int    `json:"id"`
			AuthToken    string `json:"auth_token"`
			Name         string `json:"name"`
			LastActivity string `json:"last_activity"`
		} `json:"user"`
	}{}
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return "", fmt.Errorf("can't decode response: %s", err)
	}
	return t.User.AuthToken, nil
}

func (c *Client) doRequest(path string, q map[string]string) ([]byte, error) {
	r, err := http.NewRequest("GET", "https://api.hubstaff.com"+path, nil)
	if err != nil {
		return nil, fmt.Errorf("can't create http request: %s", err)
	}

	r.Header.Set("App-Token", HSAppToken)
	r.Header.Set("Auth-Token", HSAuthToken)

	if len(q) > 0 {
		qs := r.URL.Query()
		for k, v := range q {
			qs.Add(k, v)
		}
		r.URL.RawQuery = qs.Encode()
	}
	resp, err := c.HTTPClient.Do(r)
	if err != nil {
		return nil, fmt.Errorf("can't send http request: %s", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid response code: %d", resp.StatusCode)
	}
	s, err := ioutil.ReadAll(resp.Body)
	return s, err
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
