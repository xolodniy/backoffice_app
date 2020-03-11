package app

import (
	"backoffice_app/common"
	"backoffice_app/services/jira"
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/unidoc/unioffice/spreadsheet"
)

func (a *App) WorkRatioReport(dateStart, dateEnd time.Time, channel string) {
	issues, err := a.Jira.IssuesClosedInInterim(dateStart, dateEnd)
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
		if common.ValueIn(developer, a.Slack.IgnoreList...) ||
			!common.ValueIn(issue.Fields.Type.Name, jira.TypeBETask, jira.TypeFETask, jira.TypeBESubTask, jira.TypeFESubTask, jira.TypeDesignTask) {
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
			ResolutionDate:   time.Time(issue.Fields.Resolutiondate).Format(time.RFC822Z),
			IssueLink:        fmt.Sprintf("https://atnr.atlassian.net/browse/%[1]s", issue.Key),
			IssueType:        issue.Fields.Type.Name,
			OriginalEstimate: fmt.Sprintf("%v", issue.Fields.TimeTracking.OriginalEstimateSeconds/60),
			TimeSpent:        fmt.Sprintf("%v", issue.Fields.TimeTracking.TimeSpentSeconds/60),
			DiffHours:        fmt.Sprintf("%.f", time.Duration(time.Duration(overWorkedDuration)*time.Second).Hours()),
			DiffProcent:      overWorkedDuration / (issue.Fields.TimeTracking.OriginalEstimateSeconds / 100),
		})
	}
	if err := a.CreateWorkRatioXlsxReport(workRatio, channel); err != nil {
		a.Slack.SendMessage("*Generating work reatio report was failed with err*:\n"+err.Error(), channel)
	}
}

type WorkRatioDTO struct {
	DeveloperName    string
	ResolutionDate   string
	IssueType        string
	IssueLink        string
	OriginalEstimate string
	TimeSpent        string
	DiffHours        string
	DiffProcent      int
}

// CreateWorkRatioXlsxReport create csv file with report about work ratio
func (a *App) CreateWorkRatioXlsxReport(workRatio []WorkRatioDTO, channel string) error {
	if len(workRatio) == 0 {
		a.Slack.SendMessage("There are no issues for workRatioReport.csv file", channel)
		return nil
	}
	var sheetRows [][]string
	sheetRows = append(sheetRows, []string{""}) // for unlicensed message
	sheetRows = append(sheetRows, []string{"Developer", "Resolution date", "Issue link", "Issue type", "Original estimate,h", "Time spent,h", "Diff,h", "Diff, %"})

	for _, issue := range workRatio {
		sheetRows = append(sheetRows, []string{issue.DeveloperName, issue.ResolutionDate, issue.IssueLink, issue.IssueType,
			issue.OriginalEstimate, issue.TimeSpent, issue.DiffHours, strconv.Itoa(issue.DiffProcent)})

	}
	fileName := "workRatio.xlsx"
	if err := a.CreateExcel(fileName, sheetRows); err != nil {
		a.Slack.SendMessage("*Generating work reatio report was failed with err*:\n"+err.Error(), channel)
	}
	return a.SendFileToSlack(channel, fileName)
}

// CreateExcel creates XLSX from 2-dimensional slice
func (a *App) CreateExcel(fileName string, sheetRows [][]string) error {
	ss := spreadsheet.New()
	sheet := ss.AddSheet()
	file, err := os.Create(fileName)
	if err != nil {
		logrus.WithError(err).Error("can't create file")
		return common.ErrInternal
	}
	for rowIndex, rowStrings := range sheetRows {
		row := sheet.AddNumberedRow(uint32(rowIndex + 1))
		for _, columnValue := range rowStrings {
			row.AddCell().SetString(columnValue)
		}
	}
	if err := ss.Validate(); err != nil {
		logrus.WithError(err).Error("xlsx generic form validation error")
		return common.ErrInternal
	}
	var data []byte
	buf := bytes.NewBuffer(data)
	if err := ss.Save(buf); err != nil {
		logrus.WithError(err).Error("can't write xlsx generic form")
		return common.ErrInternal
	}
	if _, err := file.Write(buf.Bytes()); err != nil {
		logrus.WithError(err).Error("can't write xlsx file")
		return common.ErrInternal
	}
	file.Close()
	return nil
}
