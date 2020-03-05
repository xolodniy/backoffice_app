package app

import (
	"backoffice_app/common"
	"backoffice_app/services/jira"
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/unidoc/unioffice/spreadsheet"
)

func (a *App) WorkRatioReport(dateStart, dateEnd, channel string) {
	dStart, err := time.Parse("02.01.2006", dateStart)
	if err != nil {
		logrus.WithError(err).WithField("dateStart", dateStart).Error("can't parse start date")
		a.Slack.SendMessage("*Generating work reatio report was failed with err*:\n"+err.Error(), channel)
		return
	}
	dEnd, err := time.Parse("02.01.2006", dateEnd)
	if err != nil {
		logrus.WithError(err).WithField("dateEnd", dateEnd).Error("can't parse end date")
		a.Slack.SendMessage("*Generating work reatio report was failed with err*:\n"+err.Error(), channel)
		return
	}
	issues, err := a.Jira.IssuesClosedInInterim(dStart, dEnd)
	if err != nil {
		a.Slack.SendMessage("*Generating work reatio report was failed with err*:\n"+err.Error(), channel)
		return
	}
	// sort by overwork %
	sort.SliceStable(issues, func(i, j int) bool {
		iEstimate := issues[i].Fields.TimeTracking.OriginalEstimateSeconds
		jEstimate := issues[j].Fields.TimeTracking.OriginalEstimateSeconds
		if iEstimate/100 == 0 || jEstimate/100 == 0 {
			return false
		}
		iTimeSpent := issues[i].Fields.TimeTracking.TimeSpentSeconds
		jTimeSpent := issues[j].Fields.TimeTracking.TimeSpentSeconds
		return (iTimeSpent-iEstimate)/(iEstimate/100) <
			(jTimeSpent - jEstimate/(jEstimate/100))
	})
	var workRatio []WorkRatioDTO
	for _, issue := range issues {
		developer := issue.DeveloperMap(jira.TagDeveloperName)
		if developer == "" {
			developer = jira.NoDeveloper
		}
		if common.ValueIn(developer, a.Slack.IgnoreList...) {
			continue
		}
		overWorkedDuration := issue.Fields.TimeTracking.TimeSpentSeconds - issue.Fields.TimeTracking.OriginalEstimateSeconds
		if overWorkedDuration < issue.Fields.TimeTracking.OriginalEstimateSeconds/10 ||
			issue.Fields.TimeTracking.RemainingEstimateSeconds != 0 ||
			issue.Fields.TimeTracking.OriginalEstimateSeconds == 0 || overWorkedDuration < 60*60 ||
			issue.Fields.TimeTracking.OriginalEstimateSeconds/100 == 0 {
			continue
		}
		workRatio = append(workRatio, WorkRatioDTO{
			DeveloperName:    developer,
			ResolutionDate:   time.Time(issue.Fields.Resolutiondate),
			IssueLink:        fmt.Sprintf("https://atnr.atlassian.net/browse/%[1]s", issue.Key),
			IssueType:        issue.Fields.Type.Name,
			OriginalEstimate: issue.Fields.TimeTracking.OriginalEstimate,
			TimeSpent:        issue.Fields.TimeTracking.TimeSpent,
			DiffHours:        time.Duration(overWorkedDuration).Hours(),
			DiffProcent:      overWorkedDuration / (issue.Fields.TimeTracking.OriginalEstimateSeconds / 100),
		})
	}
	if err := a.CreateWorkRatioCsvReport(workRatio, channel); err != nil {
		a.Slack.SendMessage("*Generating work reatio report was failed with err*:\n"+err.Error(), channel)
	}
}

type WorkRatioDTO struct {
	DeveloperName    string
	ResolutionDate   time.Time
	IssueType        string
	IssueLink        string
	OriginalEstimate string
	TimeSpent        string
	DiffHours        float64
	DiffProcent      int
}

// CreateWorkRatioCsvReport create csv file with report about work ratio
func (a *App) CreateWorkRatioCsvReport(workRatio []WorkRatioDTO, channel string) error {
	if len(workRatio) == 0 {
		a.Slack.SendMessage("There are no issues for workRatioReport.csv file", channel)
		return nil
	}
	var sheetRows [][]string
	sheetRows = append(sheetRows, []string{"Developer", "Resolution date", "Issue link", "Issue type", "Original estimate, h", "Time spent, h", "Diff, h", "Diff, %"})

	for _, issue := range workRatio {
		sheetRows = append(sheetRows, []string{issue.DeveloperName, issue.ResolutionDate.String(), issue.IssueLink, issue.IssueType,
			issue.OriginalEstimate, issue.TimeSpent, fmt.Sprintf("%.0f", issue.DiffHours), strconv.Itoa(issue.DiffProcent)})

	}
	body, err := a.CreateExcel(sheetRows)
	if err != nil {
		return err
	}
	buffer := bytes.NewBuffer(body)
	//contentType := http.DetectContentType(body)
	return a.Slack.UploadFile(channel, "application/x-www-form-urlencoded", buffer)
}

// CreateExcel creates XLSX from 2-dimensional slice
func (a *App) CreateExcel(sheetRows [][]string) ([]byte, error) {
	ss := spreadsheet.New()
	sheet := ss.AddSheet()

	for rowIndex, rowStrings := range sheetRows {
		row := sheet.AddNumberedRow(uint32(rowIndex + 1))
		for _, columnValue := range rowStrings {
			row.AddCell().SetString(columnValue)
		}
	}
	if err := ss.Validate(); err != nil {
		logrus.WithError(err).Error("xlsx generic form validation error")
		return nil, common.ErrInternal
	}
	var data []byte
	buf := bytes.NewBuffer(data)
	if err := ss.Save(buf); err != nil {
		logrus.WithError(err).Error("can't write xlsx generic form")
		return nil, common.ErrInternal
	}
	return buf.Bytes(), nil
}
