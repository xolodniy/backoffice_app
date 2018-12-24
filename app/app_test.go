package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecondsToClockTime(t *testing.T) {
	var a App

	str, err := a.SecondsToClockTime(7320)

	require.Equal(t, nil, err)
	require.Equal(t, "02:02", str)
}
