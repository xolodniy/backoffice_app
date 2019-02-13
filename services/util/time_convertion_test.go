package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDurationStringInHoursMinutes_1(t *testing.T) {

	str, err := DurationStringInHoursMinutes(86400)

	require.Equal(t, nil, err)
	require.Equal(t, "24:00", str)
}

func TestDurationStringInHoursMinutes_2(t *testing.T) {

	str, err := DurationStringInHoursMinutes(86461)

	require.Equal(t, nil, err)
	require.Equal(t, "24:01", str)
}

func TestDurationStringInHoursMinutes_3(t *testing.T) {

	str, err := DurationStringInHoursMinutes(162120)

	require.Equal(t, nil, err)
	require.Equal(t, "45:02", str)
}

func TestDurationStringInHoursMinutes_4(t *testing.T) {

	str, err := DurationStringInHoursMinutes(161943)

	require.Equal(t, nil, err)
	require.Equal(t, "44:59", str)
}

func TestDurationStringInHoursMinutes_5(t *testing.T) {
	str, err := DurationStringInHoursMinutes(842400)

	require.Equal(t, nil, err)
	require.Equal(t, "234:00", str)
}

func TestFormatDateTimeToJiraRepresentation_1(t *testing.T) {
	str, err := FormatDateTimeToJiraRepresentation(86400)
	require.Equal(t, nil, err)
	require.Equal(t, "3d", strings.Trim(str, " "))
}

func TestFormatDateTimeToJiraRepresentation_2(t *testing.T) {
	str, err := FormatDateTimeToJiraRepresentation(270240)
	require.Equal(t, nil, err)
	require.Equal(t, "1w 2d 3h 4m", strings.Trim(str, " "))
}

func TestFormatDateTimeToJiraRepresentation_3(t *testing.T) {
	str, err := FormatDateTimeToJiraRepresentation(205140)
	require.Equal(t, nil, err)
	require.Equal(t, "1w 59m", strings.Trim(str, " "))
}

func TestFormatDateTimeToJiraRepresentation_4(t *testing.T) {
	str, err := FormatDateTimeToJiraRepresentation(61200)
	require.Equal(t, nil, err)
	require.Equal(t, "2d 1h", strings.Trim(str, " "))
}

func TestFormatDateTimeToJiraRepresentation_5(t *testing.T) {
	str, err := FormatDateTimeToJiraRepresentation(259200)
	require.Equal(t, nil, err)
	require.Equal(t, "1w 2d", strings.Trim(str, " "))
}
