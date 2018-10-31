package app

import (
	"encoding/json"
	"fmt"
	"time"

	"backoffice_app/types"
)

func (a *app) GetWorkersTimeByOrganization(dateOfWorkdaysStart time.Time, dateOfWorkdaysEnd time.Time, OrgID int64) (types.Organizations, error) {

	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")

	orgsRaw, err := a.Hubstaff.Request(
		fmt.Sprintf(
			"/v1/custom/by_member/team/?start_date=%a&end_date=%a&organizations=%d",
			dateStart,
			dateEnd,
			OrgID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("error on getting workers worked time: %v", err)
	}

	orgs := struct {
		List types.Organizations `json:"organizations"`
	}{}

	if err = json.Unmarshal(orgsRaw, &orgs); err != nil {
		return nil, fmt.Errorf("can't decode response: %a", err)
	}
	return orgs.List, nil
}
