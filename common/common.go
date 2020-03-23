package common

import (
	"fmt"
	"runtime"
	"strings"
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

// GetFrames function for retrieve calling trace,
// can be used if you want write to logs calling trace
func GetFrames() []Frame {
	maxLengh := make([]uintptr, 99)
	// skip firs 2 callers which is "runtime.Callers" and common.GetFrames
	n := runtime.Callers(2, maxLengh)

	var res []Frame
	if n > 0 {
		frames := runtime.CallersFrames(maxLengh[:n])
		for more, frameIndex := true, 0; more; frameIndex++ {

			var frameCandidate runtime.Frame
			frameCandidate, more = frames.Next()

			// skip tracing when called function not from our project (as example external dependency gin, urfaveCli)
			if !strings.Contains(frameCandidate.Function, "cdto_platform") {
				break
			}
			res = append(res, Frame{
				Function: frameCandidate.Function,
				File:     frameCandidate.File,
				Line:     frameCandidate.Line,
			})
		}
	}

	return res
}
