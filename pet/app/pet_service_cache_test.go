package app_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

func TestReadPetFallsBackToRepositoryOnCacheError(t *testing.T) {
	t.Parallel()

	cache := &brokenCache{getErr: errors.New("redis refused")}
	repository := newFakeRepository()
	service := app.NewService(repository, newFakeJournal(), cache, passthroughTx{}, fixedClock{now: testNow()})
	userID := uuid.New()

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.State(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, userID, pet.UserID())
}

func TestStateReportsMissingPetWhenCacheAndRepositoryMiss(t *testing.T) {
	t.Parallel()

	cache := &brokenCache{getErr: errors.New("redis refused")}
	service := app.NewService(
		newFakeRepository(), newFakeJournal(), cache, passthroughTx{}, fixedClock{now: testNow()})

	pet, err := service.State(t.Context(), uuid.New())

	require.ErrorIs(t, err, domainerr.ErrNotFound)
	assert.Nil(t, pet)
}

func TestMutateToleratesCacheWriteFailure(t *testing.T) {
	t.Parallel()

	cache := &brokenCache{setErr: errors.New("redis full")}
	service := app.NewService(
		newFakeRepository(), newFakeJournal(), cache, passthroughTx{}, fixedClock{now: testNow()})
	userID := uuid.New()

	pet, err := service.Create(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, userID, pet.UserID())
	assert.Equal(t, 1, cache.sets)
}

func TestMutateToleratesCacheInvalidationFailure(t *testing.T) {
	t.Parallel()

	cache := &brokenCache{deleteErr: errors.New("redis gone")}
	sentinel := errors.New("commit rejected")
	service := app.NewService(
		newFakeRepository(), newFakeJournal(), cache, failingTx{err: sentinel}, fixedClock{now: testNow()})

	pet, err := service.Create(t.Context(), uuid.New())

	require.ErrorIs(t, err, sentinel)
	assert.Nil(t, pet)
	assert.Equal(t, 1, cache.deletes)
}

func TestLoadWrapsUnexpectedRepositoryError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("connection reset")
	repository := &failingLoadRepository{fakeRepository: newFakeRepository(), err: sentinel}
	service := app.NewService(
		repository, newFakeJournal(), newFakeCache(), passthroughTx{}, fixedClock{now: testNow()})

	_, err := service.Create(t.Context(), uuid.New())

	require.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "load pet")
}

func TestStateSkipsHotStateForNilStore(t *testing.T) {
	t.Parallel()

	service, _, _ := newTestService(t)
	userID := uuid.New()

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.State(t.Context(), userID)

	require.NoError(t, err)
	assert.Zero(t, pet.InteractionVersion())
}

func TestStrokeFallsBackWhenPetIsMissingFromCacheAndRepository(t *testing.T) {
	t.Parallel()

	hot := &configurableHotStore{}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)
	userID := uuid.New()

	pet, err := service.Stroke(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, userID, pet.UserID())
	assert.Zero(t, hot.strokes)
}

func TestServiceOptionsReturnSameService(t *testing.T) {
	t.Parallel()

	service, _, _ := newTestService(t)

	assert.Same(t, service, service.WithAccountGate(&stubAccountGate{active: true}))
	assert.Same(t, service, service.WithHatchNotifier(&spyHatchNotifier{}))
	assert.Same(t, service, service.WithHotState(&configurableHotStore{}))
}

func TestProgressPropagatesStateFailure(t *testing.T) {
	t.Parallel()

	service, _, _ := newTestService(t)

	view, err := service.Progress(t.Context(), uuid.New())

	require.ErrorIs(t, err, domainerr.ErrNotFound)
	assert.Zero(t, view.Level)
}

func TestRewardQualityListingAwardsXPAndRecordsEvent(t *testing.T) {
	t.Parallel()

	service, journal, _ := newTestService(t)
	userID, itemID := uuid.New(), uuid.New()

	result, err := service.RewardQualityListing(t.Context(), userID, itemID, domain.QualityListing{
		HasPhoto: true, HasPrice: true, Description: qualityDescription(),
	})

	require.NoError(t, err)
	assert.Positive(t, result.Progress.XPGranted)
	assert.Equal(t, userID, result.Pet.UserID())
	require.Len(t, journal.events, 1)
	assert.Equal(t, domain.ActionQualityListing, journal.events[0].Action)
	require.NotNil(t, journal.events[0].SubjectID)
	assert.Equal(t, itemID, *journal.events[0].SubjectID)
}

func TestRewardQualityListingRejectsLowQualityListing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		listing domain.QualityListing
	}{
		{name: "no photo", listing: domain.QualityListing{
			HasPrice: true, Description: qualityDescription(),
		}},
		{name: "no price", listing: domain.QualityListing{
			HasPhoto: true, Description: qualityDescription(),
		}},
		{name: "short description", listing: domain.QualityListing{
			HasPhoto: true, HasPrice: true, Description: "too short",
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service, journal, _ := newTestService(t)

			_, err := service.RewardQualityListing(t.Context(), uuid.New(), uuid.New(), tc.listing)

			require.ErrorIs(t, err, domain.ErrConditionNotMet)
			assert.Empty(t, journal.events)
		})
	}
}

func TestRewardQualityListingIsIdempotentPerItem(t *testing.T) {
	t.Parallel()

	service, journal, _ := newTestService(t)
	userID, itemID := uuid.New(), uuid.New()
	listing := domain.QualityListing{
		HasPhoto: true, HasPrice: true, Description: qualityDescription(),
	}

	_, err := service.RewardQualityListing(t.Context(), userID, itemID, listing)
	require.NoError(t, err)

	_, err = service.RewardQualityListing(t.Context(), userID, itemID, listing)

	require.ErrorIs(t, err, domain.ErrDuplicateAction)
	assert.Len(t, journal.events, 1)
}
