package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/pet/domain"
)

type partialActivity struct {
	*stubActivity
	rewardsErr error
	badgesErr  error
	beforeErr  error
	issuesErr  error
}

func (a *partialActivity) RewardsGranted(
	ctx context.Context, userID uuid.UUID, from, to time.Time,
) ([]string, error) {
	if a.rewardsErr != nil {
		return nil, a.rewardsErr
	}

	return a.stubActivity.RewardsGranted(ctx, userID, from, to)
}

func (a *partialActivity) BadgesEarned(
	ctx context.Context, userID uuid.UUID, from, to time.Time,
) ([]string, error) {
	if a.badgesErr != nil {
		return nil, a.badgesErr
	}

	return a.stubActivity.BadgesEarned(ctx, userID, from, to)
}

func (a *partialActivity) XPTotalBefore(
	ctx context.Context, userID uuid.UUID, at time.Time,
) (int, error) {
	if a.beforeErr != nil {
		return 0, a.beforeErr
	}

	return a.stubActivity.XPTotalBefore(ctx, userID, at)
}

func (a *partialActivity) ListingIssues(
	ctx context.Context, userID uuid.UUID, limit int,
) ([]domain.ListingIssue, error) {
	if a.issuesErr != nil {
		return nil, a.issuesErr
	}

	return a.stubActivity.ListingIssues(ctx, userID, limit)
}

type failingRank struct{ err error }

func (r failingRank) RankOf(context.Context, uuid.UUID) (rank int, found bool, err error) {
	return 0, false, r.err
}

func collectWith(t *testing.T, activity app.DayActivityReadModel, rank app.RankReadModel) (domain.DayFacts, error) {
	t.Helper()

	collector := app.NewFactCollector(app.FactCollectorDeps{
		Pets:     &stubSummaryPets{pet: petWith(3, 30, 2, nil)},
		Activity: activity,
		Rank:     rank,
	})

	return collector.Collect(t.Context(), summaryUser(), yesterday())
}

func TestCollectFailsOnRewardsError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("rewards join failed")
	activity := &partialActivity{stubActivity: &stubActivity{}, rewardsErr: sentinel}

	_, err := collectWith(t, activity, nil)

	require.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "collect granted rewards")
}

func TestCollectFailsOnBadgesError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("badges join failed")
	activity := &partialActivity{stubActivity: &stubActivity{}, badgesErr: sentinel}

	_, err := collectWith(t, activity, nil)

	require.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "collect earned badges")
}

func TestCollectKeepsCurrentLevelWhenHistoryUnavailable(t *testing.T) {
	t.Parallel()

	activity := &partialActivity{
		stubActivity: &stubActivity{before: 999},
		beforeErr:    errors.New("history partition dropped"),
	}

	facts, err := collectWith(t, activity, nil)

	require.NoError(t, err)
	assert.Equal(t, facts.Level.Current, facts.Level.Previous)
	assert.False(t, facts.LeveledUp)
}

func TestCollectSkipsIssuesWhenReadModelFails(t *testing.T) {
	t.Parallel()

	activity := &partialActivity{
		stubActivity: &stubActivity{issues: []domain.ListingIssue{{ItemID: "abc123def456"}}},
		issuesErr:    errors.New("listing index unavailable"),
	}

	facts, err := collectWith(t, activity, nil)

	require.NoError(t, err)
	assert.Empty(t, facts.Issues)
}

func TestCollectSkipsLeaderboardWhenRankFails(t *testing.T) {
	t.Parallel()

	facts, err := collectWith(t, &stubActivity{}, failingRank{err: errors.New("rank view stale")})

	require.NoError(t, err)
	assert.False(t, facts.Leaderboard.Known)
	assert.Zero(t, facts.Leaderboard.Rank)
}

func TestCollectSkipsLeaderboardWhenRankIsUnknown(t *testing.T) {
	t.Parallel()

	facts, err := collectWith(t, &stubActivity{}, stubRank{found: false})

	require.NoError(t, err)
	assert.False(t, facts.Leaderboard.Known)
}

func TestCollectIncludesRankWhenAvailable(t *testing.T) {
	t.Parallel()

	facts, err := collectWith(t, &stubActivity{}, stubRank{rank: 12, found: true})

	require.NoError(t, err)
	assert.True(t, facts.Leaderboard.Known)
	assert.Equal(t, 12, facts.Leaderboard.Rank)
}

func TestCollectWithoutRankReadModel(t *testing.T) {
	t.Parallel()

	facts, err := collectWith(t, &stubActivity{}, nil)

	require.NoError(t, err)
	assert.False(t, facts.Leaderboard.Known)
}

func TestCollectMarksLevelUpWhenXPGrewAcrossThreshold(t *testing.T) {
	t.Parallel()

	collector := app.NewFactCollector(app.FactCollectorDeps{
		Pets:     &stubSummaryPets{pet: petWith(4, 90, 2, nil)},
		Activity: &stubActivity{before: 0},
	})

	facts, err := collector.Collect(t.Context(), summaryUser(), yesterday())

	require.NoError(t, err)
	assert.True(t, facts.LeveledUp)
	assert.Less(t, facts.Level.Previous, facts.Level.Current)
}

func TestCollectAggregatesActionsAndTotalXP(t *testing.T) {
	t.Parallel()

	activity := &stubActivity{
		aggregates: []app.XPAggregate{
			{Action: domain.ActionFavorite, Count: 3, Amount: 3},
			{Action: domain.ActionItemPublished, Count: 2, Amount: 20},
		},
	}

	facts, err := collectWith(t, activity, nil)

	require.NoError(t, err)
	assert.Len(t, facts.Actions, 2)
	assert.Equal(t, 23, facts.TotalXP)
}
