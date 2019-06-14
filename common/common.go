package common

import (
	"fmt"
	"time"
)

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
