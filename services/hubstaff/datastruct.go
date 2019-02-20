package hubstaff

import (
	"fmt"
)

const (
	secInMin  = 60
	secInHour = 60 * secInMin
)

// HubstaffAuth is an object used to specifying parameters of issues searching in Hubstaff
type HubstaffAuth struct {
	Token    string
	AppToken string
	Login    string
	Password string
}

// WorkingTime type for reflect the working time and easy convert in to string format
type WorkingTime int

// String converts seconds value to 00:00 (hours with leading zero:minutes with leading zero) time format
func (wt WorkingTime) String() string {
	hours := int(wt) / secInHour
	minutes := int(wt) % secInHour / secInMin
	return fmt.Sprintf("%.2d:%.2d", hours, minutes)
}

// CustomResponse is universal struct to reflect an custom response https://developer.hubstaff.com/docs/api#!/custom
type CustomResponse struct {
	ID         int64       `json:"id"`
	Name       string      `json:"name"`
	TimeWorked WorkingTime `json:"duration"`
	Workers    []struct {
		Name       string      `json:"name"`
		TimeWorked WorkingTime `json:"duration"`
	} `json:"users"`
	Dates []struct {
		Date       string      `json:"date"`
		TimeWorked WorkingTime `json:"duration"`
		Workers    []struct {
			Name       string      `json:"name"`
			TimeWorked WorkingTime `json:"duration"`
			Projects   []struct {
				Name       string      `json:"name"`
				TimeWorked WorkingTime `json:"duration"`
				Notes      []struct {
					Description string `json:"description"`
				} `json:"notes"`
			} `json:"projects"`
		} `json:"users"`
	} `json:"dates"`
}

// User type for query Hubstaff's users https://developer.hubstaff.com/docs/api#!/users/getV1Users
type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// LastActivity type for query last activity of users
// https://developer.hubstaff.com/docs/api#!/organizations/getV1OrganizationsIdLastActivity
type LastActivity struct {
	LastTaskID    int `json:"last_task_id"`
	LastProjectID int `json:"last_project_id"`
	User          struct {
		Name string `json:"name" binding:"required"`
	} `json:"user" binding:"required"`
}

// Task type for task of user https://developer.hubstaff.com/docs/api#!/tasks/getV1TasksId
type Task struct {
	JiraKey string `json:"remote_alternate_id"`
	Summary string `json:"summary"`
}
