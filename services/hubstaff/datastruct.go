package hubstaff

// HubstaffAuth is an object used to specifying parameters of issues searching in Hubstaff
type HubstaffAuth struct {
	Token    string
	AppToken string
	Login    string
	Password string
}

// APIResponse used to reflect an api response from /by_date endpoint
type APIResponse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	TimeWorked int64  `json:"duration"`
	Workers    []struct {
		Name       string `json:"name"`
		TimeWorked int    `json:"duration"`
	} `json:"users"`
	Dates []struct {
		Date       string `json:"date"`
		TimeWorked int    `json:"duration"`
		Workers    []struct {
			Name       string `json:"name"`
			TimeWorked int    `json:"duration"`
			Projects   []struct {
				Name       string `json:"name"`
				TimeWorked int    `json:"duration"`
				Notes      []struct {
					Description string `json:"description"`
				} `json:"notes"`
			} `json:"projects"`
		} `json:"users"`
	} `json:"dates"`
}

// UsersDTO type for query Hubstaff's users
type UserDTO struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
