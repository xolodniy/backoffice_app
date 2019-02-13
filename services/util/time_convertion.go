package util

import "fmt"

const (
	secInMin  = 60
	secInHour = 60 * secInMin
	secInDay  = 8 * secInHour
	secInWeek = 7 * secInDay
)

// converts Seconds to 00:00 (hours with leading zero:minutes with leading zero) time format
func DurationStringInHoursMinutes(durationInSeconds int) (string, error) {
	if durationInSeconds < 0 {
		return "", fmt.Errorf("time can not be less than zero")
	}
	hours := durationInSeconds / secInHour
	minutes := durationInSeconds % secInHour / secInMin
	return fmt.Sprintf("%.2d:%.2d", hours, minutes), nil
}

// formats seconds durations to Jira-like time format
func FormatDateTimeToJiraRepresentation(durationInSeconds int) (string, error) {
	if durationInSeconds < 0 {
		return "", fmt.Errorf("time can not be less than zero")
	}
	var result string

	if durationInSeconds/secInWeek > 0 {
		weeks := durationInSeconds / secInWeek
		result += fmt.Sprintf("%dw ", weeks)
		durationInSeconds -= weeks * secInWeek
	}
	if durationInSeconds/secInDay > 0 {
		days := durationInSeconds / secInDay
		result += fmt.Sprintf("%dd ", days)
		durationInSeconds -= days * secInDay
	}
	if durationInSeconds/secInHour > 0 {
		hours := durationInSeconds / secInHour
		result += fmt.Sprintf("%dh ", hours)
		durationInSeconds -= hours * secInHour
	}
	if durationInSeconds/secInMin > 0 {
		result += fmt.Sprintf("%dm", durationInSeconds/secInMin)
	}
	return result, nil
}
