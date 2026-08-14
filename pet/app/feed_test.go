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

func (s *configurableHotStore) Feed(
	_ context.Context,
	initial app.HotState,
	now time.Time,
) (app.HotState, bool, error) {
	s.feeds++
	if s.feedErr != nil {
		return app.HotState{}, false, s.feedErr
	}

	initial.Satiety = min(initial.Satiety+domain.FeedSatietyGain, domain.MaxParameterValue)
	initial.Version++
	initial.UpdatedAt = now

	if s.badVersion {
		initial.Version = -1
	}

	return initial, true, nil
}

type strokeOnlyHotStore struct{ strokes int }

func (s *strokeOnlyHotStore) GetOrInitialize(
	_ context.Context,
	initial app.HotState,
) (app.HotState, error) {
	return initial, nil
}

func (s *strokeOnlyHotStore) Stroke(
	_ context.Context,
	initial app.HotState,
	now time.Time,
) (app.HotState, bool, error) {
	s.strokes++
	initial.Version++
	initial.UpdatedAt = now

	return initial, true, nil
}

func (s *strokeOnlyHotStore) DirtyBatch(context.Context, int64) ([]app.HotState, error) {
	return nil, nil
}

func (s *strokeOnlyHotStore) Acknowledge(context.Context, app.HotState) error { return nil }

func TestFeedWithoutHotStateUsesDatabase(t *testing.T) {
	t.Parallel()

	service, _, cache := newTestService(t)
	userID := uuid.New()

	pet, err := service.Feed(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, userID, pet.UserID())
	assert.Positive(t, pet.Satiety())
	assert.NotNil(t, cache.stored[userID])
}

func TestFeedUsesHotStateWhenAvailable(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	hot := &configurableHotStore{}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.Feed(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, 1, hot.feeds)
	assert.Zero(t, hot.strokes)
	assert.Positive(t, pet.InteractionVersion())
	assert.Greater(t, pet.Satiety(), 70)
}

func TestFeedFallsBackToDatabaseOnHotFailure(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	hot := &configurableHotStore{feedErr: errors.New("redis down")}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.Feed(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, 1, hot.feeds)
	assert.Equal(t, userID, pet.UserID())
	assert.Positive(t, pet.InteractionVersion())
}

func TestFeedFallsBackWhenHotStateRejected(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	hot := &configurableHotStore{badVersion: true}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.Feed(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, userID, pet.UserID())
}

func TestFeedUsesDatabaseWhenHotStoreCannotFeed(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	hot := &strokeOnlyHotStore{}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.Feed(t.Context(), userID)

	require.NoError(t, err)
	assert.Zero(t, hot.strokes)
	assert.Equal(t, userID, pet.UserID())
	assert.Positive(t, pet.Satiety())
}

func TestFeedFallsBackWhenPetIsMissingFromCacheAndRepository(t *testing.T) {
	t.Parallel()

	hot := &configurableHotStore{}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)
	userID := uuid.New()

	pet, err := service.Feed(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, userID, pet.UserID())
	assert.Zero(t, hot.feeds)
}

func TestFeedGrantsNoExperience(t *testing.T) {
	t.Parallel()

	service, journal, _ := newTestService(t)
	userID := uuid.New()

	created, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.Feed(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, created.XP(), pet.XP())
	assert.Equal(t, created.Level(), pet.Level())
	assert.Empty(t, journal.events)
}

func TestFeedClampsSatietyAtMaximum(t *testing.T) {
	t.Parallel()

	service, _, _ := newTestService(t)
	userID := uuid.New()

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	var pet *domain.Pet
	for range 5 {
		pet, err = service.Feed(t.Context(), userID)
		require.NoError(t, err)
	}

	assert.Equal(t, domain.MaxParameterValue, pet.Satiety())
}
