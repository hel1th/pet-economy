package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/games/domain"
)

func day(year int, month time.Month, d int) domain.Day {
	return domain.Day{Year: year, Month: month, Day: d}
}

func dayPtr(year int, month time.Month, d int) *domain.Day {
	value := day(year, month, d)

	return &value
}

func TestDayOfNormalizesToUTCCalendarDay(t *testing.T) {
	t.Parallel()

	moment := time.Date(2026, time.March, 14, 23, 59, 59, 0, time.UTC)

	assert.Equal(t, day(2026, time.March, 14), domain.DayOf(moment))
}

func TestDayIsDayBeforeAcrossBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		earlier  domain.Day
		later    domain.Day
		adjacent bool
	}{
		{"same month", day(2026, time.March, 14), day(2026, time.March, 15), true},
		{"month boundary", day(2026, time.March, 31), day(2026, time.April, 1), true},
		{"february to march", day(2026, time.February, 28), day(2026, time.March, 1), true},
		{"leap february", day(2024, time.February, 28), day(2024, time.February, 29), true},
		{"leap february end", day(2024, time.February, 29), day(2024, time.March, 1), true},
		{"year boundary", day(2025, time.December, 31), day(2026, time.January, 1), true},
		{"two day gap", day(2026, time.March, 14), day(2026, time.March, 16), false},
		{"same day", day(2026, time.March, 14), day(2026, time.March, 14), false},
		{"backwards", day(2026, time.March, 15), day(2026, time.March, 14), false},
		{"non leap february", day(2026, time.February, 28), day(2026, time.February, 29), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.adjacent, tc.earlier.IsDayBefore(tc.later))
		})
	}
}

func TestAdvanceStreakTransitions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		initial domain.Streak
		today   domain.Day
		want    domain.Streak
	}{
		{
			name:    "first ever completion starts at one",
			initial: domain.Streak{},
			today:   day(2026, time.March, 14),
			want: domain.Streak{
				CurrentDays: 1, BestDays: 1, LastDay: dayPtr(2026, time.March, 14),
			},
		},
		{
			name: "same day is idempotent",
			initial: domain.Streak{
				CurrentDays: 3, BestDays: 5, LastDay: dayPtr(2026, time.March, 14),
			},
			today: day(2026, time.March, 14),
			want: domain.Streak{
				CurrentDays: 3, BestDays: 5, LastDay: dayPtr(2026, time.March, 14),
			},
		},
		{
			name: "yesterday continues the streak",
			initial: domain.Streak{
				CurrentDays: 3, BestDays: 5, LastDay: dayPtr(2026, time.March, 13),
			},
			today: day(2026, time.March, 14),
			want: domain.Streak{
				CurrentDays: 4, BestDays: 5, LastDay: dayPtr(2026, time.March, 14),
			},
		},
		{
			name: "gap resets to one",
			initial: domain.Streak{
				CurrentDays: 6, BestDays: 6, LastDay: dayPtr(2026, time.March, 10),
			},
			today: day(2026, time.March, 14),
			want: domain.Streak{
				CurrentDays: 1, BestDays: 6, LastDay: dayPtr(2026, time.March, 14),
			},
		},
		{
			name: "streak continues across a month boundary",
			initial: domain.Streak{
				CurrentDays: 4, BestDays: 4, LastDay: dayPtr(2026, time.March, 31),
			},
			today: day(2026, time.April, 1),
			want: domain.Streak{
				CurrentDays: 5, BestDays: 5, LastDay: dayPtr(2026, time.April, 1),
			},
		},
		{
			name: "streak continues across a year boundary",
			initial: domain.Streak{
				CurrentDays: 2, BestDays: 9, LastDay: dayPtr(2025, time.December, 31),
			},
			today: day(2026, time.January, 1),
			want: domain.Streak{
				CurrentDays: 3, BestDays: 9, LastDay: dayPtr(2026, time.January, 1),
			},
		},
		{
			name: "streak continues across leap day",
			initial: domain.Streak{
				CurrentDays: 1, BestDays: 1, LastDay: dayPtr(2024, time.February, 29),
			},
			today: day(2024, time.March, 1),
			want: domain.Streak{
				CurrentDays: 2, BestDays: 2, LastDay: dayPtr(2024, time.March, 1),
			},
		},
		{
			name: "best days never shrinks",
			initial: domain.Streak{
				CurrentDays: 1, BestDays: 12, LastDay: dayPtr(2026, time.March, 1),
			},
			today: day(2026, time.March, 14),
			want: domain.Streak{
				CurrentDays: 1, BestDays: 12, LastDay: dayPtr(2026, time.March, 14),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := domain.AdvanceStreak(tc.initial, tc.today)

			assert.Equal(t, tc.want.CurrentDays, got.CurrentDays)
			assert.Equal(t, tc.want.BestDays, got.BestDays)
			require.NotNil(t, got.LastDay)
			assert.Equal(t, *tc.want.LastDay, *got.LastDay)
		})
	}
}

func TestAdvanceStreakDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	last := day(2026, time.March, 13)
	initial := domain.Streak{CurrentDays: 3, BestDays: 3, LastDay: &last}

	domain.AdvanceStreak(initial, day(2026, time.March, 14))

	assert.Equal(t, 3, initial.CurrentDays)
	assert.Equal(t, day(2026, time.March, 13), *initial.LastDay)
}

func TestAdvanceStreakSevenConsecutiveDaysReachesReward(t *testing.T) {
	t.Parallel()

	streak := domain.Streak{}
	current := day(2026, time.February, 25)

	for i := 0; i < domain.RewardTargetDays; i++ {
		streak = domain.AdvanceStreak(streak, current)
		current = current.AddDays(1)
	}

	assert.Equal(t, domain.RewardTargetDays, streak.CurrentDays)
	assert.True(t, domain.RewardReady(streak))
	assert.Equal(t, day(2026, time.March, 4), current)
}

func TestRewardReady(t *testing.T) {
	t.Parallel()

	assert.False(t, domain.RewardReady(domain.Streak{CurrentDays: 6}))
	assert.True(t, domain.RewardReady(domain.Streak{CurrentDays: 7}))
	assert.True(t, domain.RewardReady(domain.Streak{CurrentDays: 9}))
}

func TestClaimStreakResetsCycle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 14, 10, 0, 0, 0, time.UTC)

	claimed, ok := domain.ClaimStreak(domain.Streak{CurrentDays: 7, BestDays: 7}, now)

	require.True(t, ok)
	assert.Equal(t, 0, claimed.CurrentDays)
	assert.Equal(t, 7, claimed.BestDays)
	require.NotNil(t, claimed.RewardClaimedAt)
	assert.Equal(t, now, *claimed.RewardClaimedAt)
	assert.False(t, domain.RewardReady(claimed))
}

func TestClaimStreakRejectsIncompleteCycle(t *testing.T) {
	t.Parallel()

	initial := domain.Streak{CurrentDays: 6}

	claimed, ok := domain.ClaimStreak(initial, time.Now())

	assert.False(t, ok)
	assert.Equal(t, 6, claimed.CurrentDays)
	assert.Nil(t, claimed.RewardClaimedAt)
}

func TestAdvanceDaily(t *testing.T) {
	t.Parallel()

	progress := domain.DailyProgress{}
	assert.True(t, domain.IsFirstAttemptOfDay(progress))

	progress = domain.AdvanceDaily(progress, 7)
	assert.Equal(t, 1, progress.Attempts)
	assert.Equal(t, 7, progress.BestStreak)
	assert.False(t, domain.IsFirstAttemptOfDay(progress))

	progress = domain.AdvanceDaily(progress, 5)
	assert.Equal(t, 2, progress.Attempts)
	assert.Equal(t, 7, progress.BestStreak)

	progress = domain.AdvanceDaily(progress, 11)
	assert.Equal(t, 3, progress.Attempts)
	assert.Equal(t, 11, progress.BestStreak)
}
