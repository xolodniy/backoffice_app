package hubstaff

import (
	"backoffice_app/common"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

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

var CurrentActivityDuration int64 = 1000

func (d DateReport) String() string {
	//separatedDate print
	message := fmt.Sprintf("\n\n\n*%s*", d.Date)
	for _, worker := range d.Users {
		//employee name print
		message += fmt.Sprintf("\n\n\n*%s (%s total)*\n", worker.Name, worker.TimeWorked)
		for _, project := range worker.Projects {
			message += fmt.Sprintf("\n%s - %s", project.TimeWorked, project.Name)
			for _, task := range project.Tasks {
				message += fmt.Sprintf("\n - %s - %s (%s)", task.RemoteAlternateId, task.Summary, task.TimeWorked)
			}
			var projectNotes []string
			for _, note := range project.Notes {
				projectNotes = append(projectNotes, note.Description)
			}
			sortedNotes := removeDoubles(projectNotes)
			for _, note := range sortedNotes {
				message += fmt.Sprintf("\n ✎ %s", note)
			}
		}
	}
	return message
}

// removeDoubles removes the same strings in slice
func removeDoubles(arr []string) []string {
	for i := len(arr) - 1; i > 0; i-- {
		for j := i - 1; j >= 0; j-- {
			if strings.ToLower(arr[i]) == strings.ToLower(arr[j]) {
				arr = append(arr[:j], arr[j+1:]...)
				i = len(arr) - 1
			}
		}
	}
	return arr
}

// ObtainAuthToken retrieves auth token which must be sent along with appToken,
// see https://support.hubstaff.com/time-tracking-api/ for details
func (h *Hubstaff) ObtainAuthToken(auth HubstaffAuth) (string, error) {
	form := url.Values{}
	form.Add("email", auth.Login)
	form.Add("password", auth.Password)

	request, err := http.NewRequest("POST", h.APIURL+"/v1/auth", strings.NewReader(form.Encode()))
	if err != nil {
		logrus.WithError(err).WithField("request", request).Error("can't create http POST Request")
		return "", common.ErrInternal
	}
	if err != nil {
		return "", err
	}

	request.Header.Set("App-Token", h.AppToken)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		logrus.WithError(err).WithField("request", request).Error("can't send http POST Request")
		return "", common.ErrInternal
	}
	if response.StatusCode != 200 {
		logrus.WithError(err).WithFields(logrus.Fields{"request": request, "responseCode": response.StatusCode, "responseBody": response.Body}).
			Error("invalid response code")
		return "", common.ErrInternal
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
		logrus.WithError(err).WithFields(logrus.Fields{"request": request, "responseCode": response.StatusCode, "responseBody": response.Body}).
			Error("can't decode response")
		return "", common.ErrInternal
	}
	return t.User.AuthToken, nil
}

// Request is main API GET request method
func (h *Hubstaff) do(path string) ([]byte, error) {
	request, err := http.NewRequest("GET", h.APIURL+path, nil)
	if err != nil {
		logrus.WithError(err).WithField("path", path).Error("Can't create http request")
		return nil, common.ErrInternal
	}

	request.Header.Set("App-Token", h.AppToken)
	request.Header.Set("Auth-Token", h.AuthToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		logrus.WithError(err).WithField("request", request).Error("Can't do http request")
		return nil, common.ErrInternal
	}
	if response.StatusCode != 200 {
		logrus.WithError(err).WithFields(logrus.Fields{"request": request, "responseCode": response.StatusCode, "responseBody": response.Body}).
			Error("invalid response code")
		return nil, common.ErrInternal
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logrus.WithError(err).WithField("request", request).Error("Can't read response body")
		return nil, common.ErrInternal
	}
	return body, nil
}

// HubstaffUsers returns a slice of Hubstaff users
func (h *Hubstaff) HubstaffUsers() ([]UserReport, error) {
	apiURL := "/v1/users"
	res, err := h.do(apiURL)
	if err != nil {
		return nil, err
	}

	usersSlice := struct {
		List []UserReport `json:"users"`
	}{}

	if err = json.Unmarshal(res, &usersSlice); err != nil {
		logrus.WithError(err).WithField("res", string(res)).
			Error("can't  unmarshal response body for hubstaff users")
		return nil, common.ErrInternal
	}
	return usersSlice.List, nil
}

// CurrentActivity returns a text report about last activities
func (h *Hubstaff) CurrentActivity() ([]LastActivity, error) {
	rawResponse, err := h.do(fmt.Sprintf("/v1/organizations/%d/last_activity?include_removed=false", h.OrgID))
	if err != nil {
		return []LastActivity{}, err
	}
	activities := struct {
		List []LastActivity `json:"last_activities"`
	}{}

	if err = json.Unmarshal(rawResponse, &activities); err != nil {
		logrus.WithError(err).WithField("res", string(rawResponse)).
			Error("can't  unmarshal response body for hubstaff users")
		return []LastActivity{}, common.ErrInternal
	}
	if len(activities.List) == 0 {
		return []LastActivity{}, nil
	}
	var currentActivities []LastActivity
	for _, activity := range activities.List {
		t, err := time.Parse(time.RFC3339, activity.User.LastActivity)
		// if time empty or other format we continue to remove many log messages
		if err != nil {
			continue
		}
		lastActivity := time.Now().Unix() - t.Unix()
		if lastActivity > CurrentActivityDuration {
			continue
		}
		activity.ProjectName, err = h.getProjectNameByID(activity.LastProjectID)
		if err != nil {
			continue
		}
		if activity.LastTaskID != 0 {
			activity.TaskJiraKey, activity.TaskSummary, err = h.getJiraTaskKeyByID(activity.LastTaskID)
			if err != nil {
				continue
			}
		}
		currentActivities = append(currentActivities, activity)
	}
	return currentActivities, nil
}

func (h *Hubstaff) getProjectNameByID(projectID int) (string, error) {
	if projectID == 0 {
		return "", nil
	}
	rawResponse, err := h.do(fmt.Sprintf("/v1/projects/%d", projectID))
	if err != nil {
		return "", err
	}
	response := struct {
		Project struct {
			Name string `json:"name"`
		} `json:"project"`
	}{}

	if err = json.Unmarshal(rawResponse, &response); err != nil {
		logrus.WithError(err).WithField("res", string(rawResponse)).
			Error("can't  unmarshal response body for project name by id")
		return "", common.ErrInternal
	}
	if response.Project.Name == "" {
		logrus.WithError(err).WithField("projectID", projectID).Error("No projects have found by id")
		return "", common.ErrNotFound{"No projects have found by id"}
	}
	return response.Project.Name, nil
}

func (h *Hubstaff) getJiraTaskKeyByID(taskID int) (string, string, error) {
	if taskID == 0 {
		return "", "", nil
	}
	rawResponse, err := h.do(fmt.Sprintf("/v1/tasks/%d", taskID))
	if err != nil {
		return "", "", err
	}
	response := struct {
		Task struct {
			JiraKey string `json:"remote_alternate_id"`
			Summary string `json:"summary"`
		} `json:"task"`
	}{}

	if err = json.Unmarshal(rawResponse, &response); err != nil {
		logrus.WithError(err).WithField("res", string(rawResponse)).
			Error("can't  unmarshal response body for jira task key by id")
		return "", "", common.ErrInternal
	}
	if response.Task.JiraKey == "" {
		logrus.WithError(err).WithField("taskID", taskID).Error("No tasks have found by id")
		return "", "", common.ErrNotFound{"No tasks have found by id"}
	}
	return response.Task.JiraKey, response.Task.Summary, nil
}

// UsersWorkTimeByMember retrieves work time of user reports slice by member
func (h *Hubstaff) UsersWorkTimeByMember(dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time) ([]UserReport, error) {
	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")
	apiURL := fmt.Sprintf("/v1/custom/by_member/team/?start_date=%s&end_date=%s&organizations=%d",
		dateStart, dateEnd, h.OrgID)

	orgsRaw, err := h.do(apiURL)
	if err != nil {
		return []UserReport{}, err
	}
	orgs := struct {
		List []struct {
			Users []UserReport `json:"users"`
		} `json:"organizations"`
	}{}

	if err = json.Unmarshal(orgsRaw, &orgs); err != nil {
		logrus.WithError(err).WithField("res", string(orgsRaw)).
			Error("can't  unmarshal response body for users work time by member")
		return []UserReport{}, common.ErrInternal
	}

	if len(orgs.List) == 0 {
		logrus.WithError(err).WithFields(logrus.Fields{"dateOfWorkdaysStart": dateOfWorkdaysStart, "dateOfWorkdaysEnd": dateOfWorkdaysEnd}).
			Error("No tracked time for now or no organization found")
		return []UserReport{}, common.ErrNotFound{"No tracked time for now or no organization found"}
	}
	if len(orgs.List[0].Users) == 0 {
		logrus.WithError(err).WithFields(logrus.Fields{"dateOfWorkdaysStart": dateOfWorkdaysStart, "dateOfWorkdaysEnd": dateOfWorkdaysEnd}).
			Error("No workers found")
		return []UserReport{}, common.ErrNotFound{"No workers found"}
	}

	//get hubstaff's user list to add emails
	hubstaffUsers, err := h.HubstaffUsers()
	if err != nil {
		return []UserReport{}, err
	}
	for i, userReport := range orgs.List[0].Users {
		for _, user := range hubstaffUsers {
			if user.Name == userReport.Name {
				orgs.List[0].Users[i].Email = user.Email
				break
			}
		}
	}
	return orgs.List[0].Users, nil
}

// UsersWorkTimeByDate retrieves work time of date reports slice by date
func (h *Hubstaff) UsersWorkTimeByDate(dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time) ([]DateReport, error) {
	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")
	apiURL := fmt.Sprintf("/v1/custom/by_date/team/?start_date=%s&end_date=%s&organizations=%d&show_notes=%t&show_tasks=%t",
		dateStart, dateEnd, h.OrgID, true, true)

	orgsRaw, err := h.do(apiURL)
	if err != nil {
		return []DateReport{}, err
	}
	orgs := struct {
		List []struct {
			Dates []DateReport `json:"dates"`
		} `json:"organizations"`
	}{}

	if err = json.Unmarshal(orgsRaw, &orgs); err != nil {
		logrus.WithError(err).WithField("res", string(orgsRaw)).
			Error("can't  unmarshal response body for users work time by date")
		return []DateReport{}, common.ErrInternal
	}
	if len(orgs.List) == 0 {
		logrus.WithError(err).WithFields(logrus.Fields{"dateOfWorkdaysStart": dateOfWorkdaysStart, "dateOfWorkdaysEnd": dateOfWorkdaysEnd}).
			Error("No tracked time for now or no organization found")
		return []DateReport{}, common.ErrNotFound{"No tracked time for now or no organization found"}
	}
	if len(orgs.List[0].Dates) == 0 {
		logrus.WithError(err).WithFields(logrus.Fields{"dateOfWorkdaysStart": dateOfWorkdaysStart, "dateOfWorkdaysEnd": dateOfWorkdaysEnd}).
			Error("No tracked time for now found")
		return []DateReport{}, common.ErrNotFound{"No tracked time for now found"}
	}
	return orgs.List[0].Dates, nil
}

// UserWorkTimeByDate retrieves work time of user date report slice by date and retrieve user name
func (h *Hubstaff) UserWorkTimeByDate(dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time, email string) (DateReport, error) {
	users, err := h.HubstaffUsers()
	if err != nil {
		return DateReport{}, err
	}
	var userName string
	for _, user := range users {
		if user.Email == email {
			userName = user.Name
			break
		}
	}
	if userName == "" {
		logrus.WithError(err).WithFields(logrus.Fields{"dateOfWorkdaysStart": dateOfWorkdaysStart, "dateOfWorkdaysEnd": dateOfWorkdaysEnd, "email": email}).
			Error("User was not found in Hubstaff by email")
		return DateReport{}, common.ErrNotFound{"User was not found in Hubstaff by email"}
	}
	dateReports, err := h.UsersWorkTimeByDate(dateOfWorkdaysStart, dateOfWorkdaysEnd)
	if err != nil {
		return DateReport{}, err
	}
	var userWorkReport DateReport
	for _, dateReport := range dateReports {
		for _, user := range dateReport.Users {
			if user.Name == userName {
				userWorkReport.Users = append(userWorkReport.Users, user)
			}
		}
	}
	return userWorkReport, nil
}

// LastUserNote returns last user note for last 12 hours
func (h *Hubstaff) LastUserNote(userID, projectID string) (Note, error) {
	params := url.Values{}
	// get all user notes for last 12 hours
	params.Add("start_time", time.Now().Add(-12*time.Hour).Format(time.RFC3339))
	params.Add("stop_time", time.Now().Format(time.RFC3339))
	params.Add("users", userID)
	params.Add("projects", projectID)
	rawResponse, err := h.do(fmt.Sprintf("/v1/notes/?%s", params.Encode()))
	if err != nil {
		return Note{}, err
	}
	notesList := struct {
		Notes []Note `json:"notes"`
	}{}

	if err = json.Unmarshal(rawResponse, &notesList); err != nil {
		logrus.WithError(err).WithField("res", string(rawResponse)).
			Error("can't  unmarshal response body for last user note")
		return Note{}, common.ErrInternal
	}
	if len(notesList.Notes) == 0 {
		return Note{}, nil
	}
	var lastUserNote Note
	for _, note := range notesList.Notes {
		if note.RecordedAt.After(lastUserNote.RecordedAt) {
			lastUserNote = note
		}
	}
	return lastUserNote, nil
}
