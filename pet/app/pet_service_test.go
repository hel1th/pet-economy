package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/pet/domain"
)

type spyHatchNotifier struct {
	mu      sync.Mutex
	hatched []uuid.UUID
}

func (n *spyHatchNotifier) PetHatched(userID uuid.UUID, _ *domain.Pet) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.hatched = append(n.hatched, userID)
}

type configurableHotStore struct {
	strokeErr  error
	feedErr    error
	initErr    error
	badVersion bool
	strokes    int
	feeds      int
}

func (s *configurableHotStore) GetOrInitialize(_ context.Context, initial app.HotState) (app.HotState, error) {
	if s.initErr != nil {
		return app.HotState{}, s.initErr
	}

	return initial, nil
}

func (s *configurableHotStore) Stroke(
	_ context.Context,
	initial app.HotState,
	now time.Time,
) (app.HotState, bool, error) {
	s.strokes++
	if s.strokeErr != nil {
		return app.HotState{}, false, s.strokeErr
	}

	initial.Happiness = min(initial.Happiness+5, 100)
	initial.Version++
	initial.UpdatedAt = now

	if s.badVersion {
		initial.Version = -1
	}

	return initial, true, nil
}

func (s *configurableHotStore) DirtyBatch(context.Context, int64) ([]app.HotState, error) {
	return nil, nil
}

func (s *configurableHotStore) Acknowledge(context.Context, app.HotState) error { return nil }

func TestStrokeWithoutHotStateUsesDatabase(t *testing.T) {
	t.Parallel()

	service, _, cache := newTestService(t)
	userID := uuid.New()

	pet, err := service.Stroke(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, userID, pet.UserID())
	assert.Positive(t, pet.Happiness())
	assert.NotNil(t, cache.stored[userID])
}

func TestStrokeUsesHotStateWhenAvailable(t *testing.T) {
	t.Parallel()

	service, _, _ := newTestService(t)
	_, err := service.Create(t.Context(), uuid.New())
	require.NoError(t, err)

	userID := uuid.New()
	hot := &configurableHotStore{}
	service, _, _ = newTestService(t)
	service = service.WithHotState(hot)

	_, err = service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.Stroke(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, 1, hot.strokes)
	assert.Positive(t, pet.InteractionVersion())
}

func TestStrokeFallsBackToDatabaseOnHotFailure(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	hot := &configurableHotStore{strokeErr: errors.New("redis down")}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.Stroke(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, 1, hot.strokes)
	assert.Equal(t, userID, pet.UserID())
}

func TestStrokeFallsBackWhenHotStateRejected(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	hot := &configurableHotStore{badVersion: true}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.Stroke(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, userID, pet.UserID())
}

func TestStateToleratesHotStateFailure(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	hot := &configurableHotStore{initErr: errors.New("redis unavailable")}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.State(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, userID, pet.UserID())
}

func TestNewPetsNeverTriggerHatchNotification(t *testing.T) {
	t.Parallel()

	notifier := &spyHatchNotifier{}
	service, _, _ := newTestService(t)
	service = service.WithHatchNotifier(notifier)
	userID := uuid.New()

	pet, err := service.Stroke(t.Context(), userID)
	require.NoError(t, err)
	require.True(t, pet.IsHatched())

	_, err = service.Stroke(t.Context(), userID)
	require.NoError(t, err)
	assert.Empty(t, notifier.hatched)
}

func TestCreateInitializesPetForUnknownUser(t *testing.T) {
	t.Parallel()

	service, _, _ := newTestService(t)
	userID := uuid.New()

	pet, err := service.Create(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, userID, pet.UserID())
	assert.Equal(t, 1, pet.Level())
	assert.Equal(t, domain.StageBaby, pet.Stage())
	assert.True(t, pet.IsHatched())
}

func TestCheckInAwardsXPAndStreak(t *testing.T) {
	t.Parallel()

	service, journal, _ := newTestService(t)
	userID := uuid.New()

	result, err := service.CheckIn(t.Context(), userID)

	require.NoError(t, err)
	assert.Positive(t, result.Progress.XPGranted)
	assert.Equal(t, 1, result.Streak.Days)
	assert.Len(t, journal.events, 1)
}

func TestCheckInIsIdempotentWithinSameDay(t *testing.T) {
	t.Parallel()

	service, _, _ := newTestService(t)
	userID := uuid.New()

	_, err := service.CheckIn(t.Context(), userID)
	require.NoError(t, err)

	_, err = service.CheckIn(t.Context(), userID)
	assert.ErrorIs(t, err, domain.ErrDuplicateAction)
}

func TestProgressReflectsPetState(t *testing.T) {
	t.Parallel()

	service, _, _ := newTestService(t)
	userID := uuid.New()

	_, err := service.CheckIn(t.Context(), userID)
	require.NoError(t, err)

	view, err := service.Progress(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, 1, view.StreakDays)
	assert.Positive(t, view.NextLevelXP)
	assert.False(t, view.IsMaxLevel)
}
