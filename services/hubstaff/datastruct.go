package hubstaff

// HubstaffAuth is an object used to specifying parameters of issues searching in Hubstaff
type HubstaffAuth struct {
	Token    string
	AppToken string
	Login    string
	Password string
}

// CustomResponse is universal struct to reflect an custom response https://developer.hubstaff.com/docs/api#!/custom
type CustomResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Duration int    `json:"duration"`
	Users    []struct {
		Name     string `json:"name"`
		Duration int    `json:"duration"`
		Email    string `json:"email"`
	} `json:"users"`
	Dates []struct {
		Date     string `json:"date"`
		Duration int    `json:"duration"`
		Workers  []struct {
			Name     string `json:"name"`
			Duration int    `json:"duration"`
			Projects []struct {
				Name     string `json:"name"`
				Duration int    `json:"duration"`
				Notes    []struct {
					Description string `json:"description"`
				} `json:"notes"`
			} `json:"projects"`
		} `json:"users"`
	} `json:"dates"`
	LastActivities []struct {
		LastTaskID    int `json:"last_task_id"`
		LastProjectID int `json:"last_project_id"`
		User          struct {
			Name string `json:"name" binding:"required"`
		} `json:"user" binding:"required"`
	} `json:"last_activities"`
	Task struct {
		JiraKey string `json:"remote_alternate_id"`
		Summary string `json:"summary"`
	} `json:"task"`
	Project struct {
		Name string `json:"name"`
	} `json:"project"`
	User struct {
		AuthToken string `json:"auth_token"`
	} `json:"user"`
}
