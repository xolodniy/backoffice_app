package hubstaff

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"backoffice_app/config"
	"backoffice_app/libs"
	"backoffice_app/types"
)

type service struct {
	Client *libs.HubStaff
}

func New(config config.HubStaff) (*service, error) {
	client := &libs.HubStaff{
		HTTPClient: http.DefaultClient,
		AppToken:   config.Auth.AppToken,
	}

	if err := client.Authorize(config.Auth); err != nil {
		return nil, err
	}

	return &service{client}, nil

}

func (HubStaff *service) GetWorkersTimeByOrganization(dateOfWorkdaysStart time.Time, dateOfWorkdaysEnd time.Time, OrgID int64) (types.Organizations, error) {

	var dateStart = dateOfWorkdaysStart.Format("2006-01-02")
	var dateEnd = dateOfWorkdaysEnd.Format("2006-01-02")

	orgsRaw, err := HubStaff.Client.Request(
		fmt.Sprintf(
			"/v1/custom/by_member/team/?start_date=%s&end_date=%s&organizations=%d",
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
		return nil, fmt.Errorf("can't decode response: %s", err)
	}
	return orgs.List, nil
}
