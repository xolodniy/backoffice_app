package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"backoffice_app/types"
)

var AppToken = "yWDG5mMG3yln_GaIg-P5vnvlKlWeXZC9IE9cqAuDkoQ"
var Login = ""
var Password = ""
var AuthToken = ""
var OursOrgsID = 60470
var SlackOutToken = ""
var ChannelID = ""
var BotName = "Я чьё угодно имя"

type Client struct {
	// AppToken created at https://developer.hubstaff.com/my_apps
	AppToken string

	// (optional) AuthToken, previously obtained through ObtainAuthToken
	AuthToken string

	// HTTPClient is required to be passed. Pass http.DefaultClient if not sure
	HTTPClient *http.Client
}
func main() {
	hubstaff := Client{
		AppToken:   AppToken,
		AuthToken:  AuthToken, // Set it if already known. If not, see below how to obtain it.
		HTTPClient: http.DefaultClient,
	}

	if AuthToken == "" || AuthToken == "..." {
		authToken, err := hubstaff.ObtainAuthToken(Login, Password)
		hubstaff.AuthToken = authToken
		fmt.Print(authToken, err)
		os.Exit(2)
	}


	var date = time.Now().Format("2006-01-02")
	orgsRaw, err := hubstaff.doRequest(fmt.Sprintf("/v1/custom/by_member/team/?start_date=%s&end_date=%s&organizations=%d", 	date, date, OursOrgsID), nil)
	if err != nil {
		fmt.Print( err )
		os.Exit( 4 )
		return
	}

	orgs := struct {
		List types.Organizations `json:"organizations"`
	}{}
	if err := json.Unmarshal(orgsRaw, &orgs); err != nil {
		fmt.Print(fmt.Errorf("can't decode response: %s", err))
		return
	}

	if len(orgs.List) == 0 {
		if err := sendStandardMessage("No tracked time for now or no organization found"); err != nil {
			fmt.Print( err )
		}
		os.Exit(5)
	}

	var concatenatedString string
	for workerListOrderID, worker := range orgs.List[0].Workers {
		concatenatedString += fmt.Sprintf(
			"%d. %s — %s\n",
			workerListOrderID+1,
			secondsToClockTime( worker.TimeWorked ),
			worker.Name,
		)
	}

	if concatenatedString == "" {
		concatenatedString = "No tracked time for now or no workers found"
	}

	if err := sendStandardMessage(concatenatedString); err != nil {
		fmt.Print( err )
		return
	}
	os.Exit(0)
}

func secondsToClockTime(seconds int ) (string) {

	hours, minutes := math.Modf(float64(120) / 60 / 60)

	var Hours string
	if int(hours) < 10 {
		Hours = fmt.Sprintf("0%d", int(hours))
	} else {
		Hours = fmt.Sprintf("%d", int(hours))
	}

	var Minutes string
	if int(math.Round(minutes*60)) < 10 {
		Minutes = fmt.Sprintf("0%d", int(math.Round(minutes*60)))
	} else {
		Minutes = fmt.Sprintf("%d", int(math.Round(minutes*60)))
	}

	return fmt.Sprintf(
		"%s:%s", Hours, Minutes,
	)

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
	r.Header.Set("App-Token", AppToken)
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

	r.Header.Set("App-Token", AppToken)
	r.Header.Set("Auth-Token", AuthToken)

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
	var msg = types.NewPostChannelMessage(text, channelID, asUser, username, SlackOutToken)
	fmt.Printf("NewPostChannelMessage is:\n%+v\n", msg)

	return sendPOSTMessage(msg)
}
func sendStandardMessage( message string ) error {
	_, err := postChannelMessage(
		message,
		ChannelID,
		false,
		BotName,
	)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return err
	}
	return nil
}