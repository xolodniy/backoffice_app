package hubstaff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoveDoubles_1(t *testing.T) {
	unsortedSlice := []string{"Test", "troll", "Remove DOUBLES", "remove doubles", "Troll", "test"}
	sortedSlice := removeDoubles(unsortedSlice)
	require.Equal(t, []string{"remove doubles", "Troll", "test"}, sortedSlice)
}

func TestRemoveDoubles_2(t *testing.T) {
	unsortedSlice := []string{"Fill", "Fill", "Crontab", "Crontab", "Pull Request", "Pull Request"}
	sortedSlice := removeDoubles(unsortedSlice)
	require.Equal(t, []string{"Fill", "Crontab", "Pull Request"}, sortedSlice)
}

func TestRemoveDoubles_3(t *testing.T) {
	unsortedSlice := []string{"True", "false", "cancel", "improvement", "later", "donation"}
	sortedSlice := removeDoubles(unsortedSlice)
	require.Equal(t, []string{"True", "false", "cancel", "improvement", "later", "donation"}, sortedSlice)
}

func TestRemoveDoubles_4(t *testing.T) {
	unsortedSlice := []string{"True"}
	sortedSlice := removeDoubles(unsortedSlice)
	require.Equal(t, []string{"True"}, sortedSlice)
}

func TestRemoveDoubles_5(t *testing.T) {
	unsortedSlice := []string{"TEST", "test", "tEst", "teSt", "tesT", "Test"}
	sortedSlice := removeDoubles(unsortedSlice)
	require.Equal(t, []string{"Test"}, sortedSlice)
}

func TestRemoveDoubles_6(t *testing.T) {
	unsortedSlice := []string{"TEST", "PYthon", "test", "PyThon", "tEst", "PytHOn", "teSt", "PythoN", "tesT", "PYTHON", "Test", "Python"}
	sortedSlice := removeDoubles(unsortedSlice)
	require.Equal(t, []string{"Test", "Python"}, sortedSlice)
}
