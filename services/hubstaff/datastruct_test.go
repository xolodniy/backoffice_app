package hubstaff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDurationStringInHoursMinutes_1(t *testing.T) {
	wt := WorkingTime(86400)
	str := wt.String()
	require.Equal(t, "24:00", str)
}

func TestDurationStringInHoursMinutes_2(t *testing.T) {
	wt := WorkingTime(86461)
	str := wt.String()
	require.Equal(t, "24:01", str)
}

func TestDurationStringInHoursMinutes_3(t *testing.T) {
	wt := WorkingTime(162120)
	str := wt.String()
	require.Equal(t, "45:02", str)
}

func TestDurationStringInHoursMinutes_4(t *testing.T) {
	wt := WorkingTime(161943)
	str := wt.String()
	require.Equal(t, "44:59", str)
}

func TestDurationStringInHoursMinutes_5(t *testing.T) {
	wt := WorkingTime(842400)
	str := wt.String()
	require.Equal(t, "234:00", str)
}
