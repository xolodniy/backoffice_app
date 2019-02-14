package util

import (
	"fmt"
)

const (
	secInMin  = 60
	secInHour = 60 * secInMin
	secInDay  = 8 * secInHour
	secInWeek = 7 * secInDay
)

// WorkingTime type for reflect the working time and easy convert in to string format
type WorkingTime int

func (wt *WorkingTime) String() string {
	return DurationStringInHoursMinutes(int(*wt))
}

// StringGracefull returns gracefully formatted duration
func (wt *WorkingTime) StringGracefull() string {
	return DurationStringGracefull(int(*wt))
}

// DurationStringInHoursMinutes converts Seconds to 00:00 (hours with leading zero:minutes with leading zero) time format
func DurationStringInHoursMinutes(durationInSeconds int) string {
	hours := durationInSeconds / secInHour
	minutes := durationInSeconds % secInHour / secInMin
	return fmt.Sprintf("%.2d:%.2d", hours, minutes)
}

// DurationStringGracefull formats seconds durations to Jira-like time format (1w 2d 3h 4m)
func DurationStringGracefull(durationInSeconds int) string {
	var result string
	if durationInSeconds < secInMin {
		return "0m"
	}

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
	return result
}
