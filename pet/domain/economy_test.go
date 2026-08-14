package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLevelProgressionAndRewards(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	pet.Hatch(testTime())
	progress := pet.awardXP(420)

	assert.Equal(t, MaxLevel, pet.Level())
	assert.Equal(t, StageLegend, pet.Stage())
	assert.Equal(t, 0, pet.NextLevelXP())
	assert.Len(t, progress.UnlockedRewards, 14)
	assert.Equal(t, "free_delivery_and_avito_guru_badge", progress.UnlockedRewards[13])
}

func TestDailyCheckIn(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	for dayNumber := range 7 {
		_, err := pet.DailyCheckIn(testTime().AddDate(0, 0, dayNumber))
		require.NoError(t, err)
	}

	assert.Equal(t, 7, pet.StreakDays())
	_, err := pet.DailyCheckIn(testTime().AddDate(0, 0, 6).Add(time.Hour))
	require.ErrorIs(t, err, ErrDuplicateAction)
}

func TestDailyCheckInGrantsMilestoneBonus(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	var granted int
	for dayNumber := range 7 {
		progress, err := pet.DailyCheckIn(testTime().AddDate(0, 0, dayNumber))
		require.NoError(t, err)
		granted = progress.XPGranted
	}

	assert.Greater(t, granted, 30)
}

func TestDailyCheckInResetsBrokenStreak(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	_, err := pet.DailyCheckIn(testTime())
	require.NoError(t, err)
	_, err = pet.DailyCheckIn(testTime().AddDate(0, 0, 2))
	require.NoError(t, err)

	assert.Equal(t, 1, pet.StreakDays())
}

func TestLimitedActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action LimitedAction
		call   func(*Pet, LimitedAction) (Progress, error)
		err    error
		xp     int
	}{
		{name: "favorite", action: validLimitedAction(), call: (*Pet).RewardFavorite, xp: 1},
		{
			name: "favorite duplicate", action: actionWithUnique(false), call: (*Pet).RewardFavorite,
			err: ErrDuplicateAction,
		},
		{
			name: "favorite limit", action: actionWithCount(5), call: (*Pet).RewardFavorite,
			err: ErrLimitReached,
		},
		{name: "view", action: validLimitedAction(), call: (*Pet).RewardItemViewed, xp: 1},
		{
			name: "view limit", action: actionWithCount(MaxItemViewsPerDay),
			call: (*Pet).RewardItemViewed, err: ErrLimitReached,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pet := New(uuid.New(), testTime())
			progress, err := tt.call(pet, tt.action)
			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.xp, progress.XPGranted)
		})
	}
}

func TestContentAndTrustRewards(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	progress, err := pet.RewardQualityListing(QualityListing{
		HasPhoto: true, HasPrice: true,
		Description: strings.Repeat("я", 201), Now: testTime(),
	})
	require.NoError(t, err)
	assert.Equal(t, 3, progress.XPGranted)
}

func TestRewardQualityListingRules(t *testing.T) {
	t.Parallel()

	longDescription := strings.Repeat("я", 201)

	tests := []struct {
		name    string
		listing QualityListing
		wantXP  int
		err     error
	}{
		{
			name:    "photo, price and description are enough",
			listing: QualityListing{HasPhoto: true, HasPrice: true, Description: longDescription},
			wantXP:  3,
		},
		{
			name:    "no photo is rejected",
			listing: QualityListing{HasPrice: true, Description: longDescription},
			err:     ErrConditionNotMet,
		},
		{
			name:    "no price is rejected",
			listing: QualityListing{HasPhoto: true, Description: longDescription},
			err:     ErrConditionNotMet,
		},
		{
			name:    "short description is rejected",
			listing: QualityListing{HasPhoto: true, HasPrice: true, Description: strings.Repeat("я", 200)},
			err:     ErrConditionNotMet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pet := New(uuid.New(), testTime())
			listing := tt.listing
			listing.Now = testTime()

			progress, err := pet.RewardQualityListing(listing)
			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.wantXP, progress.XPGranted)
		})
	}
}

func validLimitedAction() LimitedAction {
	return LimitedAction{SubjectID: uuid.New(), Unique: true, Now: testTime()}
}

func actionWithUnique(unique bool) LimitedAction {
	action := validLimitedAction()
	action.Unique = unique
	return action
}

func actionWithCount(count int) LimitedAction {
	action := validLimitedAction()
	action.RewardedCount = count
	return action
}

func testTime() time.Time {
	return time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
}
