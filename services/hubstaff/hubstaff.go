package hubstaff

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"backoffice_app/config"
)

// Hubstaff is main Hubstaff implementation
type Hubstaff struct {
	APIURL    string
	AppToken  string
	AuthToken string
	OrgID     int64
}

// New creates new Hubstaff
func New(config *config.Hubstaff) Hubstaff {
	return Hubstaff{
		AppToken:  config.Auth.AppToken,
		AuthToken: config.Auth.Token,
		APIURL:    config.APIURL,
		OrgID:     config.OrgsID,
	}
}

// ObtainAuthToken retrieves auth token which must be sent along with appToken,
// see https://support.hubstaff.com/time-tracking-api/ for details
func (h *Hubstaff) ObtainAuthToken(auth HubstaffAuth) (string, error) {
	form := url.Values{}
	form.Add("email", auth.Login)
	form.Add("password", auth.Password)

	request, err := http.NewRequest("POST", h.APIURL+"/v1/auth", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("can't create http POST Request: %s", err)
	}

	request.Header.Set("App-Token", h.AppToken)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(request)
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
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("can't send http Request: %s", err)
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("invalid response code: %d", response.StatusCode)
	}
	s, err := ioutil.ReadAll(response.Body)
	return s, err
}

// TimeLogs returning parsed workers timelogs
func (h *Hubstaff) TimeLogs(apiURL string) (CustomResponse, error) {
	res := CustomResponse{}
	orgsRaw, err := h.Request(apiURL, nil)

	if err != nil {
		return res, fmt.Errorf("error on getting workers worked time: %v", err)
	}

	orgs := struct {
		List []CustomResponse `json:"organizations"`
	}{}

	if err = json.Unmarshal(orgsRaw, &orgs); err != nil {
		return res, fmt.Errorf("can't decode response: %s", err)
	}

	if len(orgs.List) == 0 {
		return res, fmt.Errorf("No tracked time for now or no organization found")
	}
	if len(orgs.List[0].Workers) == 0 && len(orgs.List[0].Dates) == 0 {
		return res, fmt.Errorf("No tracked time for now or no workers found")
	}
	return orgs.List[0], nil
}

// Users returns a slice of Hubstaff users
func (h *Hubstaff) Users() ([]User, error) {
	apiURL := "/v1/users"
	orgsRaw, err := h.Request(apiURL, nil)

	if err != nil {
		return nil, fmt.Errorf("error on getting workers list: %v", err)
	}

	usersSlice := struct {
		List []User `json:"users"`
	}{}

	if err = json.Unmarshal(orgsRaw, &usersSlice); err != nil {
		return nil, fmt.Errorf("can't decode response: %s", err)
	}
	return usersSlice.List, nil
}

// GetLastActivityReport returns a text report about last activities
func (h *Hubstaff) LastActivities() ([]LastActivity, error) {
	rawResponse, err := h.Request(fmt.Sprintf("/v1/organizations/%d/last_activity", h.OrgID), nil)
	if err != nil {
		return []LastActivity{}, fmt.Errorf("error on getting last activities list: %v", err)
	}
	activities := struct {
		List []LastActivity `json:"last_activities"`
	}{}

	if err = json.Unmarshal(rawResponse, &activities); err != nil {
		return []LastActivity{}, fmt.Errorf("can't decode response: %s", err)
	}
	return activities.List, nil
}

// ProjectName retrieves project name by id
func (h *Hubstaff) ProjectName(projectID int) (string, error) {
	if projectID == 0 {
		return "", nil
	}
	rawResponse, err := h.Request(fmt.Sprintf("/v1/projects/%d", projectID), nil)
	if err != nil {
		return "", fmt.Errorf("error on getting priject name: %v", err)
	}
	response := struct {
		Project struct {
			Name string `json:"name"`
		} `json:"project"`
	}{}

	if err = json.Unmarshal(rawResponse, &response); err != nil {
		return "", fmt.Errorf("can't decode response: %s", err)
	}
	if response.Project.Name == "" {
		return "", fmt.Errorf("No projects have found by id: %d", projectID)
	}

	return response.Project.Name, nil
}

// JiraTask retrieves jira task by task id
func (h *Hubstaff) JiraTask(taskID int) (Task, error) {
	if taskID == 0 {
		return Task{}, nil
	}
	rawResponse, err := h.Request(fmt.Sprintf("/v1/tasks/%d", taskID), nil)
	if err != nil {
		return Task{}, fmt.Errorf("error on getting jira task: %v", err)
	}
	response := struct {
		Task Task `json:"task"`
	}{}

	if err = json.Unmarshal(rawResponse, &response); err != nil {
		return Task{}, fmt.Errorf("can't decode response: %s", err)
	}
	if response.Task.JiraKey == "" {
		return Task{}, fmt.Errorf("No tasks have found by id: %d", taskID)
	}

	return response.Task, nil
}

// WorkersWorkedTime retrieves workers worked time
func (h *Hubstaff) WorkersWorkedTime(dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time) (CustomResponse, error) {
	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")

	apiURL := fmt.Sprintf("/v1/custom/by_member/team/?start_date=%s&end_date=%s&organizations=%d",
		dateStart, dateEnd, h.OrgID)
	return h.TimeLogs(apiURL)
}

// WorkersWorkedTimeDetailed retrieves detailed workers worked time
func (h *Hubstaff) WorkersWorkedTimeDetailed(dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time) (CustomResponse, error) {
	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")

	apiURL := fmt.Sprintf("/v1/custom/by_date/team/?start_date=%s&end_date=%s&organizations=%d&show_notes=%t",
		dateStart, dateEnd, h.OrgID, true)
	return h.TimeLogs(apiURL)
}
