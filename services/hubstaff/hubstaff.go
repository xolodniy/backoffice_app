package hubstaff

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"backoffice_app/config"
	"backoffice_app/types"
)

// Hubstaff is main Hubstaff implementation
type Hubstaff struct {
	APIURL     string
	AppToken   string
	AuthToken  string
	HTTPClient *http.Client
}

// New creates new Hubstaff
func New(config *config.Hubstaff) Hubstaff {
	return Hubstaff{
		HTTPClient: http.DefaultClient,
		AppToken:   config.Auth.AppToken,
		AuthToken:  config.Auth.Token,
		APIURL:     config.APIURL,
	}
}

// ObtainAuthToken retrieves auth token which must be sent along with appToken,
// see https://support.hubstaff.com/time-tracking-api/ for details
func (h *Hubstaff) ObtainAuthToken(auth types.HubstaffAuth) (string, error) {
	form := url.Values{}
	form.Add("email", auth.Login)
	form.Add("password", auth.Password)

	request, err := http.NewRequest("POST", h.APIURL+"/v1/auth", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("can't create http POST Request: %s", err)
	}
	if err != nil {
		return "", err
	}

	request.Header.Set("App-Token", h.AppToken)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, err := h.HTTPClient.Do(request)
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

// Request is main API GET request method
func (h *Hubstaff) Request(path string, q map[string]string) ([]byte, error) {
	request, err := http.NewRequest("GET", h.APIURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("can't create http GET Request: %s", err)
	}

	request.Header.Set("App-Token", h.AppToken)
	request.Header.Set("Auth-Token", h.AuthToken)

	if len(q) > 0 {
		qs := request.URL.Query()
		for k, v := range q {
			qs.Add(k, v)
		}
		request.URL.RawQuery = qs.Encode()
	}
	response, err := h.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("can't send http Request: %s", err)
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("invalid response code: %d", response.StatusCode)
	}
	s, err := ioutil.ReadAll(response.Body)
	return s, err
}
