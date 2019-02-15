package util

import "fmt"

const (
	secInMin  = 60
	secInHour = 60 * secInMin
)

// SecondsToHoursMinutes converts duration in seconds to hours:minutes string, like 14:40
func SecondsToHoursMinutes(duration int) string {
	hours := duration / secInHour
	minutes := duration % secInHour / secInMin
	return fmt.Sprintf("%.2d:%.2d", hours, minutes)
}
