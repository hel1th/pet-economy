package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreakIncrementsDaily(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	for dayNumber := range 3 {
		_, err := pet.CheckIn(testTime().AddDate(0, 0, dayNumber))
		require.NoError(t, err)
	}

	assert.Equal(t, 3, pet.StreakDays())
}

func TestStreakMilestoneGrantsBonusAndFreeze(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	var last CheckInResult
	for dayNumber := range 3 {
		result, err := pet.CheckIn(testTime().AddDate(0, 0, dayNumber))
		require.NoError(t, err)
		last = result
	}

	assert.Equal(t, 3, last.Streak.MilestoneReached)
	assert.Equal(t, 30, last.Streak.MilestoneBonus)
	assert.Equal(t, 1, pet.Freezes())
}

func TestStreakBreaksWithoutFreeze(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	_, err := pet.CheckIn(testTime())
	require.NoError(t, err)

	result, err := pet.CheckIn(testTime().AddDate(0, 0, 3))
	require.NoError(t, err)

	assert.True(t, result.Streak.Reset)
	assert.Equal(t, 1, pet.StreakDays())
}

func TestStreakSurvivesOneMissedDayWithFreeze(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	for dayNumber := range 3 {
		_, err := pet.CheckIn(testTime().AddDate(0, 0, dayNumber))
		require.NoError(t, err)
	}
	require.Equal(t, 1, pet.Freezes())

	result, err := pet.CheckIn(testTime().AddDate(0, 0, 4))
	require.NoError(t, err)

	assert.True(t, result.Streak.FreezeUsed)
	assert.Equal(t, 4, pet.StreakDays())
	assert.Equal(t, 0, pet.Freezes())
}

func TestCheckInIsIdempotentWithinDay(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	_, err := pet.CheckIn(testTime())
	require.NoError(t, err)

	_, err = pet.CheckIn(testTime().Add(2 * time.Hour))
	assert.ErrorIs(t, err, ErrDuplicateAction)
}

func TestFreezeCapped(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	for range 5 {
		pet.GrantFreeze()
	}

	assert.Equal(t, maxFreezes, pet.Freezes())
}
