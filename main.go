package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Client struct {
	// AppToken created at https://developer.hubstaff.com/my_apps
	AppToken string

	// (optional) AuthToken, previously obtained through ObtainAuthToken
	AuthToken string

	// HTTPClient is required to be passed. Pass http.DefaultClient if not sure
	HTTPClient *http.Client
}
type Project struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	LastActivity string `json:"timezone"` /*"2018-09-05T20:26:44.837Z"*/
	Status       string `json:"status"`
	Description  string `json:"description"`
}
type Projects []Project

var AppToken = "yWDG5mMG3yln_GaIg-P5vnvlKlWeXZC9IE9cqAuDkoQ"
var Login = "@gmail.com"
var Password = ""
var AuthToken = ""
var OursOrgsID = 60470

func main() {
	hs := Client{
		AppToken:   AppToken,
		AuthToken:  AuthToken, // Set it if already known. If not, see below how to obtain it.
		HTTPClient: http.DefaultClient,
	}

	if AuthToken == "" || AuthToken == "..." {
		authToken, err := hs.ObtainAuthToken(Login, Password)
		hs.AuthToken = authToken
		fmt.Print(authToken, err)
		os.Exit(2)
	}

	projects, err := hs.OrganizationProjects(OursOrgsID)
	if err != nil {
		fmt.Println(err)
		os.Exit(3)
	}

	for key, project := range projects {
		fmt.Println(key, project)
	}
	os.Exit(0)
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
	r.Header.Set("App-Token", c.AppToken)
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

func (c *Client) OrganizationProjects(orgID int) (Projects, error) {
	bodyRaw, err := c.doRequest("/v1/organizations/"+strconv.Itoa(orgID)+"/projects", nil)
	if err != nil {
		return nil, err
	}

	bodyUnmarshaled := struct {
		Projects Projects `json:"projects"`
	}{}
	if err := json.Unmarshal(bodyRaw, &bodyUnmarshaled); err != nil {
		return nil, fmt.Errorf("can't decode response: %s", err)
	}
	return bodyUnmarshaled.Projects, nil
}

func (c *Client) doRequest(path string, q map[string]string) ([]byte, error) {
	r, err := http.NewRequest("GET", "https://api.hubstaff.com"+path, nil)
	if err != nil {
		return nil, fmt.Errorf("can't create http request: %s", err)
	}

	r.Header.Set("App-Token", c.AppToken)
	r.Header.Set("Auth-Token", c.AuthToken)

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
