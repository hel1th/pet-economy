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

type fakeDayActivity struct {
	counts map[domain.Action]int
	err    error
}

func (a *fakeDayActivity) XPByAction(
	_ context.Context,
	_ uuid.UUID,
	_, _ time.Time,
) ([]app.XPAggregate, error) {
	if a.err != nil {
		return nil, a.err
	}

	aggregates := make([]app.XPAggregate, 0, len(a.counts))
	for action, count := range a.counts {
		aggregates = append(aggregates, app.XPAggregate{Action: action, Count: count, Amount: count})
	}

	return aggregates, nil
}

func (a *fakeDayActivity) XPTotalBefore(context.Context, uuid.UUID, time.Time) (int, error) {
	return 0, nil
}

func (a *fakeDayActivity) RewardsGranted(
	context.Context, uuid.UUID, time.Time, time.Time,
) ([]string, error) {
	return nil, nil
}

func (a *fakeDayActivity) BadgesEarned(
	context.Context, uuid.UUID, time.Time, time.Time,
) ([]string, error) {
	return nil, nil
}

func (a *fakeDayActivity) ListingIssues(
	context.Context, uuid.UUID, int,
) ([]domain.ListingIssue, error) {
	return nil, nil
}

func newQuestService(
	t *testing.T,
	userID uuid.UUID,
	counts map[domain.Action]int,
) (*app.QuestService, *fakeJournal, *fakeRepository) {
	t.Helper()

	journal := newFakeJournal()
	pets := newFakeRepository()
	require.NoError(t, pets.Save(context.Background(), domain.New(userID, testNow())))

	service := app.NewQuestService(app.QuestServiceDeps{
		Activity: &fakeDayActivity{counts: counts},
		Journal:  journal,
		Pets:     pets,
		Tx:       passthroughTx{},
		Clock:    fixedClock{now: testNow()},
	})

	return service, journal, pets
}

func TestQuestsTodayAreStableAcrossCalls(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	service, _, _ := newQuestService(t, userID, nil)

	first, err := service.Today(t.Context(), userID)
	require.NoError(t, err)

	second, err := service.Today(t.Context(), userID)
	require.NoError(t, err)

	require.Len(t, first, domain.QuestsPerDay)
	for i := range first {
		assert.Equal(t, first[i].Quest.ID, second[i].Quest.ID)
	}
}

func TestQuestProgressComesFromRecordedEvents(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	quests := domain.DailyQuests(userID, testNow())
	tracked := quests[0]

	service, _, _ := newQuestService(t, userID, map[domain.Action]int{tracked.Action: 1})

	views, err := service.Today(t.Context(), userID)
	require.NoError(t, err)

	for _, view := range views {
		if view.Quest.ID != tracked.ID {
			continue
		}
		assert.Equal(t, 1, view.Current, "progress must follow the xp journal")

		return
	}

	t.Fatalf("quest %s missing from today's list", tracked.ID)
}

func TestQuestRewardIsGrantedOnlyOnce(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	quests := domain.DailyQuests(userID, testNow())
	completed := quests[0]

	service, journal, pets := newQuestService(
		t, userID, map[domain.Action]int{completed.Action: completed.Target},
	)

	first, err := service.ClaimCompleted(t.Context(), userID)
	require.NoError(t, err)

	pet, err := pets.ByUserID(t.Context(), userID)
	require.NoError(t, err)
	awarded := pet.XP()
	assert.Positive(t, awarded, "completing a quest must grant xp")

	claimed := 0
	for _, view := range first {
		if view.Claimed {
			claimed++
		}
	}
	assert.Positive(t, claimed)

	rewardEvents := len(journal.events)

	_, err = service.ClaimCompleted(t.Context(), userID)
	require.NoError(t, err)

	pet, err = pets.ByUserID(t.Context(), userID)
	require.NoError(t, err)

	assert.Equal(t, awarded, pet.XP(), "re-claiming must not grant xp twice")
	assert.Len(t, journal.events, rewardEvents, "re-claiming must not append new reward events")
}

func TestIncompleteQuestGrantsNothing(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	service, journal, pets := newQuestService(t, userID, nil)

	views, err := service.ClaimCompleted(t.Context(), userID)
	require.NoError(t, err)

	for _, view := range views {
		assert.False(t, view.Claimed)
		assert.False(t, view.Completed)
	}

	pet, err := pets.ByUserID(t.Context(), userID)
	require.NoError(t, err)

	assert.Zero(t, pet.XP())
	assert.Empty(t, journal.events)
}

func TestQuestSubjectIDIsStablePerUserQuestAndDay(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	morning := testNow()
	evening := morning.Add(6 * time.Hour)

	assert.Equal(
		t,
		app.QuestSubjectID(userID, "favorite_three", morning),
		app.QuestSubjectID(userID, "favorite_three", evening),
	)
	assert.NotEqual(
		t,
		app.QuestSubjectID(userID, "favorite_three", morning),
		app.QuestSubjectID(userID, "favorite_three", morning.AddDate(0, 0, 1)),
	)
	assert.NotEqual(
		t,
		app.QuestSubjectID(userID, "favorite_three", morning),
		app.QuestSubjectID(uuid.New(), "favorite_three", morning),
	)
}
