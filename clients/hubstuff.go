package clients

import (
	"backoffice_app/types"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type HubStaff struct {
	APIUrl string

	// HSAppToken created at https://developer.hubstaff.com/my_apps
	AppToken string

	// (optional) HSAuthToken, previously obtained through obtainAuthToken
	AuthToken string

	// HTTPClient is required to be passed. Pass http.DefaultClient if not sure
	HTTPClient *http.Client
}

// Retrieves auth token which must be sent along with appToken,
// see https://support.hubstaff.com/time-tracking-api/ for details
func (c *HubStaff) Authorize(auth types.HubStaffAuth) error {
	if c.AuthToken == "" {
		authToken, err := c.obtainAuthToken(auth)
		if err != nil {
			return err
		}
		c.AuthToken = authToken
	}

	return nil
}

// Retrieves auth token which must be sent along with appToken,
// see https://support.hubstaff.com/time-tracking-api/ for details
func (c *HubStaff) obtainAuthToken(auth types.HubStaffAuth) (string, error) {
	form := url.Values{}
	form.Add("email", auth.Login)
	form.Add("password", auth.Password)

	request, err := c.requestPost("/v1/auth", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}

	request.Header.Set("App-Token", c.AppToken)
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

func (c *HubStaff) Request(path string, q map[string]string) ([]byte, error) {
	request, err := c.requestGet(path)
	if err != nil {
		return nil, err
	}

	request.Header.Set("App-Token", c.AppToken)
	request.Header.Set("Auth-Token", c.AuthToken)

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

func (c *HubStaff) requestGet(relativePath string) (*http.Request, error) {
	r, err := http.NewRequest("GET", c.APIUrl+relativePath, nil)
	if err != nil {
		return nil, fmt.Errorf("can't create http GET Request: %s", err)
	}
	return r, nil
}

func (c *HubStaff) requestPost(relativePath string, body io.Reader) (*http.Request, error) {
	r, err := http.NewRequest("POST", c.APIUrl+relativePath, body)
	if err != nil {
		return nil, fmt.Errorf("can't create http POST Request: %s", err)
	}
	return r, nil
}
