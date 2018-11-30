package app

import (
	"encoding/json"
	"fmt"
	"time"

	"backoffice_app/types"
)

// GetWorkersTimeByOrganization returning workers times by organization id
func (a *App) GetWorkersTimeByOrganization(dateOfWorkdaysStart, dateOfWorkdaysEnd time.Time, OrgID int64) (types.Organizations, error) {

	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")

	apiURL := fmt.Sprintf(
		"/v1/custom/by_member/team/?start_date=%s&end_date=%s&organizations=%d",
		dateStart,
		dateEnd,
		OrgID)

	orgsRaw, err := a.Hubstaff.Request(
		apiURL,
		nil,
	)

	fmt.Println("Hubstuff request URL will be:", apiURL)

	if err != nil {
		return nil, fmt.Errorf("error on getting workers worked time: %v", err)
	}

	orgs := struct {
		List types.Organizations `json:"organizations"`
	}{}

	if err = json.Unmarshal(orgsRaw, &orgs); err != nil {
		return nil, fmt.Errorf("can't decode response: %s", err)
	}
	return orgs.List, nil
}
