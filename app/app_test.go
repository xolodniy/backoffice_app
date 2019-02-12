package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDurationStringInHoursMinutes_1(t *testing.T) {
	var a App

	str, err := a.DurationStringInHoursMinutes(86400)

	require.Equal(t, nil, err)
	require.Equal(t, "24:00", str)
}

func TestDurationStringInHoursMinutes_2(t *testing.T) {
	var a App

	str, err := a.DurationStringInHoursMinutes(86461)

	require.Equal(t, nil, err)
	require.Equal(t, "24:01", str)
}

func TestDurationStringInHoursMinutes_3(t *testing.T) {
	var a App

	str, err := a.DurationStringInHoursMinutes(162120)

	require.Equal(t, nil, err)
	require.Equal(t, "45:02", str)
}

func TestDurationStringInHoursMinutes_4(t *testing.T) {
	var a App

	str, err := a.DurationStringInHoursMinutes(161943)

	require.Equal(t, nil, err)
	require.Equal(t, "44:59", str)
}

func TestDurationStringInHoursMinutes_5(t *testing.T) {
	var a App

	str, err := a.DurationStringInHoursMinutes(842400)

	require.Equal(t, nil, err)
	require.Equal(t, "234:00", str)
}
