package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"backoffice_app/config"
)

const baseAPIURL = "https://api.hubstaff.com"

type Slack struct {
	// HSAppToken created at https://developer.hubstaff.com/my_apps
	InToken string

	// (optional) HSAuthToken, previously obtained through obtainAuthToken
	OutToken string
}

// Retrieves auth token which must be sent along with appToken,
// see https://support.hubstaff.com/time-tracking-api/ for details
func (c *Slack) Authorize(auth config.HubStaffAuth) error {
	if c.OutToken == "" {
		authToken, err := c.obtainAuthToken(auth)
		if err != nil {
			return err
		}
		c.OutToken = authToken
	}

	return nil
}

// Retrieves auth token which must be sent along with appToken,
// see https://support.hubstaff.com/time-tracking-api/ for details
func (c *Slack) obtainAuthToken(auth config.HubStaffAuth) (string, error) {
	form := url.Values{}
	form.Add("email", auth.Login)
	form.Add("password", auth.Password)

	request, err := c.requestPost("/v1/auth", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}

	request.Header.Set("App-Token", c.InToken)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("can't send http Request: %s", err)
	}
	if response.StatusCode != 200 {
		return "", fmt.Errorf("invalid response code: %d", response.StatusCode)
	}
	defer response.Body.Close()

	t := struct {
		User struct {
			ID           int    `json:"id"`
			AuthToken    string `json:"auth_token"`
			Name         string `json:"name"`
			LastActivity string `json:"last_activity"`
		} `json:"user"`
	}{}
	if err := json.NewDecoder(response.Body).Decode(&t); err != nil {
		return "", fmt.Errorf("can't decode response: %s", err)
	}
	return t.User.AuthToken, nil
}

func (c *Slack) Request(path string, q map[string]string) ([]byte, error) {
	request, err := c.requestGet(path)
	if err != nil {
		return nil, err
	}

	request.Header.Set("App-Token", c.InToken)
	request.Header.Set("Auth-Token", c.OutToken)

	if len(q) > 0 {
		qs := request.URL.Query()
		for k, v := range q {
			qs.Add(k, v)
		}
		request.URL.RawQuery = qs.Encode()
	}
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("can't send http Request: %s", err)
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("invalid response code: %d", response.StatusCode)
	}
	s, err := ioutil.ReadAll(response.Body)
	return s, err
}

func (c *Slack) requestGet(relativePath string) (*http.Request, error) {
	r, err := http.NewRequest("GET", baseAPIURL+relativePath, nil)
	if err != nil {
		return nil, fmt.Errorf("can't create http GET Request: %s", err)
	}
	return r, nil
}

func (c *Slack) requestPost(relativePath string, body io.Reader) (*http.Request, error) {
	r, err := http.NewRequest("POST", baseAPIURL+relativePath, body)
	if err != nil {
		return nil, fmt.Errorf("can't create http POST Request: %s", err)
	}
	return r, nil
}
