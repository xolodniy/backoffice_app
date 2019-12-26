package common

import (
	"fmt"
	"time"
)

// FmtDuration retrive duration in string format by day, hour, minutes
func FmtDuration(duration time.Duration) string {
	d := duration / (24 * time.Hour)
	duration -= d * (24 * time.Hour)
	h := duration / time.Hour
	duration -= h * time.Hour
	m := duration / time.Minute

	switch {
	case d > 0:
		return fmt.Sprintf("%dd%02dh%02dm", d, h, m)
	case h > 0:
		return fmt.Sprintf("%dh%02dm", h, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}

// ValueIn returns true if `in` contains `value`
func ValueIn(value string, in ...string) bool {
	for _, el := range in {
		if el == value {
			return true
		}
	}
	return false
}

// RemoveDuplicates returns elements without duplicate
func RemoveDuplicates(elements []string) []string {
	result := []string{}
	for i := 0; i < len(elements); i++ {
		exists := false
		for v := 0; v < i; v++ {
			if elements[v] == elements[i] {
				exists = true
				break
			}
		}
		if !exists {
			result = append(result, elements[i])
		}
	}
	return result
}
