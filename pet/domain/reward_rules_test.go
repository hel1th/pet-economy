package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func petAt(level, streak int) *Pet {
	return Restore(RestoreParams{
		ID: uuid.New(), UserID: uuid.New(), Stage: StageBaby, Level: level,
		StreakDays: streak, Satiety: 70, Happiness: 70, Energy: 100,
		LastDecayTime: testTime(), UpdatedAt: testTime(),
	})
}

func mustReward(t *testing.T, id string, condition ConditionType, value int) Reward {
	t.Helper()

	reward, err := NewReward(RewardParams{
		ID: id, Title: id, Kind: RewardKindPromo, ConditionType: condition, ConditionValue: value,
	})
	require.NoError(t, err)

	return reward
}

func TestRewardConditionMet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition ConditionType
		target    int
		level     int
		streak    int
		want      bool
	}{
		{name: "level below target", condition: ConditionLevel, target: 5, level: 4, want: false},
		{name: "level exactly at target", condition: ConditionLevel, target: 5, level: 5, want: true},
		{name: "level above target", condition: ConditionLevel, target: 5, level: 12, want: true},
		{name: "streak below target", condition: ConditionStreak, target: 7, streak: 6, want: false},
		{name: "streak exactly at target", condition: ConditionStreak, target: 7, streak: 7, want: true},
		{name: "achievement is never met", condition: ConditionAchievement, target: 1, level: 15, want: false},
		{name: "zero target is always met", condition: ConditionLevel, target: 0, level: 1, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reward := mustReward(t, "reward", tc.condition, tc.target)

			assert.Equal(t, tc.want, RewardConditionMet(reward, petAt(tc.level, tc.streak)))
		})
	}
}

func TestRewardConditionMetHandlesNilPet(t *testing.T) {
	t.Parallel()

	reward := mustReward(t, "reward", ConditionLevel, 1)

	assert.False(t, RewardConditionMet(reward, nil))
}

func TestRewardConditionMetRejectsUnknownCondition(t *testing.T) {
	t.Parallel()

	reward := Reward{id: "x", conditionType: "karma", conditionValue: 0}

	assert.False(t, RewardConditionMet(reward, petAt(15, 30)))
}

func TestEvaluateRewardProgress(t *testing.T) {
	t.Parallel()

	locked := mustReward(t, "locked", ConditionLevel, 10)
	unlocked := mustReward(t, "unlocked", ConditionStreak, 3)
	pet := petAt(7, 5)

	lockedProgress := EvaluateRewardProgress(locked, pet)
	assert.False(t, lockedProgress.Unlocked)
	assert.Equal(t, 7, lockedProgress.Current)
	assert.Equal(t, 10, lockedProgress.Target)
	assert.Equal(t, "locked", lockedProgress.Reward.ID())

	unlockedProgress := EvaluateRewardProgress(unlocked, pet)
	assert.True(t, unlockedProgress.Unlocked)
	assert.Equal(t, 5, unlockedProgress.Current)
	assert.Equal(t, 3, unlockedProgress.Target)
}

func TestEligibleRewards(t *testing.T) {
	t.Parallel()

	rewards := []Reward{
		mustReward(t, "level_3", ConditionLevel, 3),
		mustReward(t, "level_10", ConditionLevel, 10),
		mustReward(t, "streak_7", ConditionStreak, 7),
		mustReward(t, "achievement", ConditionAchievement, 1),
	}

	eligible := EligibleRewards(rewards, petAt(5, 10))

	require.Len(t, eligible, 2)
	assert.Equal(t, "level_3", eligible[0].ID())
	assert.Equal(t, "streak_7", eligible[1].ID())

	assert.Empty(t, EligibleRewards(rewards, nil))
	assert.Empty(t, EligibleRewards(nil, petAt(15, 30)))
}

func TestRewardIDForLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level int
		want  string
	}{
		{name: "below range", level: 0, want: ""},
		{name: "negative", level: -3, want: ""},
		{name: "first level", level: 1, want: "starter_status"},
		{name: "mid level", level: 10, want: "free_delivery_500"},
		{name: "max level", level: MaxLevel, want: "free_delivery_and_avito_guru_badge"},
		{name: "above max", level: MaxLevel + 1, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, RewardIDForLevel(tc.level))
		})
	}
}

func TestBadgesEarnedBy(t *testing.T) {
	t.Parallel()

	assert.Empty(t, BadgesEarnedBy(BadgeStats{}))
	assert.Empty(t, BadgesEarnedBy(BadgeStats{Level: 9}))
	assert.Contains(t, BadgesEarnedBy(BadgeStats{Level: 10}), "raccoon_friend")
	assert.Contains(t, BadgesEarnedBy(BadgeStats{Level: MaxLevel}), "raccoon_friend")
	assert.Contains(t, BadgesEarnedBy(BadgeStats{StreakDays: 14}), "charged_streak")
	assert.Contains(
		t,
		BadgesEarnedBy(BadgeStats{Actions: map[Action]int{ActionFavorite: 5}}),
		"explorer",
	)
}

func TestBadgeAccessors(t *testing.T) {
	t.Parallel()

	badge := NewBadge("first_deal", "First deal", "Closed the first deal", "/icons/first.png")

	assert.Equal(t, "first_deal", badge.ID())
	assert.Equal(t, "First deal", badge.Name())
	assert.Equal(t, "Closed the first deal", badge.Description())
	assert.Equal(t, "/icons/first.png", badge.IconURL())

	userID := uuid.New()
	earnedAt := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	earned := NewEarnedBadge(badge, userID, earnedAt)

	assert.Equal(t, badge, earned.Badge())
	assert.Equal(t, userID, earned.UserID())
	assert.Equal(t, earnedAt, earned.EarnedAt())
	assert.Equal(t, "first_deal", earned.ID())
	assert.Equal(t, "First deal", earned.Name())
	assert.Equal(t, "Closed the first deal", earned.Description())
	assert.Equal(t, "/icons/first.png", earned.IconURL())
}

func TestStageValidity(t *testing.T) {
	t.Parallel()

	for _, stage := range []Stage{StageBaby, StageTeen, StageAdult, StageLegend} {
		assert.True(t, stage.Valid(), string(stage))
	}

	assert.False(t, Stage("").Valid())
	assert.False(t, Stage("egg").Valid())
	assert.False(t, Stage("ancient").Valid())
}
