package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func neutralPet() *Pet {
	pet := Restore(RestoreParams{
		ID: uuid.New(), UserID: uuid.New(), Stage: StageBaby, Level: 1,
		Satiety: 50, Happiness: 50, Energy: 100,
		LastDecayTime: testTime(), UpdatedAt: testTime(),
	})
	pet.Hatch(testTime())

	return pet
}

func TestAwardXPZeroAndNegativeAmounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		amount int
		wantXP int
	}{
		{name: "zero grants nothing", amount: 0, wantXP: 0},
		{name: "negative subtracts raw amount", amount: -5, wantXP: -5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pet := neutralPet()
			progress := pet.applyAward(tc.amount, testTime())

			assert.Equal(t, tc.wantXP, progress.XPGranted)
			assert.Equal(t, tc.wantXP, pet.XP())
			assert.Equal(t, 1, pet.Level())
			assert.Empty(t, progress.UnlockedRewards)
		})
	}
}

func TestAwardXPStopsAtMaxLevel(t *testing.T) {
	t.Parallel()

	pet := neutralPet()
	progress := pet.applyAward(1_000_000, testTime())

	assert.Equal(t, MaxLevel, pet.Level())
	assert.True(t, pet.IsMaxLevel())
	assert.True(t, progress.IsMaxLevel)
	assert.Zero(t, pet.NextLevelXP())
	assert.Equal(t, StageLegend, pet.Stage())
	assert.Len(t, progress.UnlockedRewards, MaxLevel-1)
	assert.Equal(t, "free_delivery_and_avito_guru_badge", progress.UnlockedRewards[MaxLevel-2])
}

func TestAwardXPAtMaxLevelKeepsAccumulatingXP(t *testing.T) {
	t.Parallel()

	pet := neutralPet()
	pet.applyAward(1_000_000, testTime())
	before := pet.XP()

	progress := pet.applyAward(100, testTime())

	assert.Equal(t, MaxLevel, pet.Level())
	assert.Equal(t, MaxLevel, progress.PreviousLevel)
	assert.Equal(t, before+progress.XPGranted, pet.XP())
	assert.Empty(t, progress.UnlockedRewards)
}

func TestStageForLevelBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		level int
		want  Stage
	}{
		{level: 1, want: StageBaby},
		{level: 5, want: StageBaby},
		{level: 6, want: StageTeen},
		{level: 9, want: StageTeen},
		{level: 10, want: StageAdult},
		{level: 14, want: StageAdult},
		{level: MaxLevel, want: StageLegend},
		{level: MaxLevel + 5, want: StageLegend},
	}

	for _, tc := range tests {
		t.Run(string(tc.want), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, stageForLevel(tc.level))
		})
	}
}

func TestNextThreshold(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 5, nextThreshold(1))
	assert.Equal(t, 12, nextThreshold(2))
	assert.Equal(t, 420, nextThreshold(MaxLevel-1))
	assert.Zero(t, nextThreshold(MaxLevel))
	assert.Zero(t, nextThreshold(MaxLevel+1))
}

func TestAwardXPCrossesMultipleLevelsAtOnce(t *testing.T) {
	t.Parallel()

	pet := neutralPet()
	progress := pet.applyAward(35, testTime())

	assert.Equal(t, 1, progress.PreviousLevel)
	assert.Equal(t, 5, progress.Level)
	assert.Equal(t, []string{
		"raccoon_accessory", "services_discount_10", "attentive_badge",
		"listing_badge_or_delivery_discount",
	}, progress.UnlockedRewards)
	assert.Equal(t, 52, progress.NextLevelXP)
}

func TestDayAndWeekStart(t *testing.T) {
	t.Parallel()

	wednesday := time.Date(2026, time.August, 5, 21, 30, 0, 0, time.UTC)

	dayStart := DayStart(wednesday)
	assert.Equal(t, 0, dayStart.Hour())
	assert.Equal(t, 6, dayStart.Day())

	weekStart := WeekStart(wednesday)
	assert.Equal(t, time.Monday, weekStart.Weekday())
	assert.True(t, weekStart.Before(dayStart))
	assert.InDelta(t, 3.0, dayStart.Sub(weekStart).Hours()/24, 0.001)
}

func TestNewXPEvent(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	subjectID := uuid.New()

	event := NewXPEvent(userID, ActionFavorite, &subjectID, 5, testTime())

	assert.NotEqual(t, uuid.Nil, event.ID)
	assert.Equal(t, userID, event.UserID)
	assert.Equal(t, ActionFavorite, event.Action)
	require.NotNil(t, event.SubjectID)
	assert.Equal(t, subjectID, *event.SubjectID)
	assert.Equal(t, 5, event.Amount)
	assert.Equal(t, testTime(), event.CreatedAt)

	assert.Nil(t, NewXPEvent(userID, ActionDailyCheckIn, nil, 1, testTime()).SubjectID)
}
