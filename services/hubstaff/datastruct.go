package hubstaff

import "backoffice_app/services/util"

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
	return util.SecondsToHoursMinutes(int(wt))
}

// APIResponse used to reflect an api response from /by_date endpoint
type APIResponse struct {
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
