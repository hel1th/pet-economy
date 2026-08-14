package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/pet/domain"
)

func summaryUser() uuid.UUID {
	return uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
}

func yesterday() time.Time {
	return domain.DayStart(time.Date(2026, time.March, 9, 12, 0, 0, 0, time.UTC))
}

func petWith(level, xp, streak int, lastCheckIn *time.Time) *domain.Pet {
	hatched := yesterday().Add(-72 * time.Hour)

	return domain.Restore(domain.RestoreParams{
		ID: uuid.New(), UserID: summaryUser(), Name: "Тоша", Stage: domain.StageBaby,
		Level: level, XP: xp, Satiety: 60, Happiness: 65, Energy: 80,
		StreakDays: streak, LastCheckInDate: lastCheckIn, HatchedAt: &hatched,
		LastDecayTime: yesterday(), UpdatedAt: yesterday(),
	})
}

func TestFactCollectorEmptyDay(t *testing.T) {
	t.Parallel()

	collector := app.NewFactCollector(app.FactCollectorDeps{
		Pets:     &stubSummaryPets{pet: petWith(2, 6, 0, nil)},
		Activity: &stubActivity{before: 6},
	})

	facts, err := collector.Collect(context.Background(), summaryUser(), yesterday())

	require.NoError(t, err)
	assert.True(t, facts.IsEmpty())
	assert.Zero(t, facts.TotalXP)
	assert.False(t, facts.LeveledUp)
	assert.Equal(t, "Тоша", facts.PetName)
	assert.Equal(t, yesterday(), facts.Date)
}

func TestFactCollectorActiveDay(t *testing.T) {
	t.Parallel()

	checkIn := yesterday()
	collector := app.NewFactCollector(app.FactCollectorDeps{
		Pets: &stubSummaryPets{pet: petWith(3, 14, 4, &checkIn)},
		Activity: &stubActivity{
			before: 14,
			aggregates: []app.XPAggregate{
				{Action: domain.ActionQualityListing, Count: 2, Amount: 4},
				{Action: domain.ActionFavorite, Count: 3, Amount: 3},
			},
			rewards: []string{"Скидка 10%"},
			badges:  []string{"Внимательный"},
		},
		Rank: stubRank{rank: 7, found: true},
	})

	facts, err := collector.Collect(context.Background(), summaryUser(), yesterday())

	require.NoError(t, err)
	assert.False(t, facts.IsEmpty())
	assert.Equal(t, 7, facts.TotalXP)
	assert.Equal(t, 2, facts.ActionCount(domain.ActionQualityListing))
	assert.Equal(t, []string{"Скидка 10%"}, facts.Rewards)
	assert.Equal(t, []string{"Внимательный"}, facts.Badges)
	assert.True(t, facts.Leaderboard.Known)
	assert.Equal(t, 7, facts.Leaderboard.Rank)
	assert.True(t, facts.Streak.CheckedIn)
	assert.False(t, facts.Streak.Broken)
	assert.Equal(t, 4, facts.Streak.Days)
}

func TestFactCollectorDetectsLevelUp(t *testing.T) {
	t.Parallel()

	checkIn := yesterday()
	collector := app.NewFactCollector(app.FactCollectorDeps{
		Pets: &stubSummaryPets{pet: petWith(4, 25, 2, &checkIn)},
		Activity: &stubActivity{
			before:     6,
			aggregates: []app.XPAggregate{{Action: domain.ActionQualityListing, Count: 8, Amount: 19}},
		},
	})

	facts, err := collector.Collect(context.Background(), summaryUser(), yesterday())

	require.NoError(t, err)
	assert.True(t, facts.LeveledUp)
	assert.Equal(t, 2, facts.Level.Previous)
	assert.Equal(t, 4, facts.Level.Current)
}

func TestFactCollectorDetectsBrokenStreak(t *testing.T) {
	t.Parallel()

	stale := yesterday().AddDate(0, 0, -3)
	collector := app.NewFactCollector(app.FactCollectorDeps{
		Pets:     &stubSummaryPets{pet: petWith(2, 6, 1, &stale)},
		Activity: &stubActivity{},
	})

	facts, err := collector.Collect(context.Background(), summaryUser(), yesterday())

	require.NoError(t, err)
	assert.True(t, facts.Streak.Broken)
	assert.False(t, facts.Streak.CheckedIn)
}

func TestFactCollectorCollectsListingIssues(t *testing.T) {
	t.Parallel()

	itemID := "abc123def456"
	collector := app.NewFactCollector(app.FactCollectorDeps{
		Pets: &stubSummaryPets{pet: petWith(2, 6, 0, nil)},
		Activity: &stubActivity{
			issues: []domain.ListingIssue{
				{ItemID: itemID, Title: "iPhone 12", Kind: domain.IssueNoPhoto},
			},
		},
	})

	facts, err := collector.Collect(context.Background(), summaryUser(), yesterday())

	require.NoError(t, err)
	require.Len(t, facts.Issues, 1)

	issue, ok := facts.TopIssue()
	require.True(t, ok)
	assert.Equal(t, domain.IssueNoPhoto, issue.Kind)
	assert.Equal(t, itemID, issue.ItemID)
}
