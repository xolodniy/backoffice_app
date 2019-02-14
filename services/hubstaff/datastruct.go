package hubstaff

// HubstaffAuth is an object used to specifying parameters of issues searching in Hubstaff
type HubstaffAuth struct {
	Token    string
	AppToken string
	Login    string
	Password string
}

type apiResponse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	TimeWorked int64  `json:"duration"`
}

// APIResponseByMember  used to reflect an api response from /by_member endpoint
type APIResponseByMember struct {
	apiResponse
	Workers []struct {
		Name       string `json:"name"`
		TimeWorked int    `json:"duration"`
	} `json:"users"`
}

// APIResponseByDate used to reflect an api response from /by_date endpoint
type APIResponseByDate struct {
	apiResponse
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
