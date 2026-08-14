package app_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
)

type serializingTx struct {
	mu sync.Mutex
}

func (t *serializingTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return fn(ctx)
}

func TestStateViewAppliesCheckInOnFirstReadOfTheDay(t *testing.T) {
	t.Parallel()

	service, journal, _ := newTestService(t)
	userID := uuid.New()

	view, err := service.StateView(t.Context(), userID)

	require.NoError(t, err)
	assert.True(t, view.CheckInApplied)
	assert.Positive(t, view.Progress.XPGranted)
	assert.Equal(t, 1, view.Streak.Days)
	assert.Len(t, journal.events, 1)
	require.NotNil(t, view.Pet)
	assert.Equal(t, userID, view.Pet.UserID())
}

func TestStateViewIsIdempotentWithinTheSameDay(t *testing.T) {
	t.Parallel()

	service, journal, _ := newTestService(t)
	userID := uuid.New()

	first, err := service.StateView(t.Context(), userID)
	require.NoError(t, err)
	require.True(t, first.CheckInApplied)

	second, err := service.StateView(t.Context(), userID)

	require.NoError(t, err)
	assert.False(t, second.CheckInApplied)
	assert.Zero(t, second.Progress.XPGranted)
	assert.Zero(t, second.Streak.Days)
	assert.Len(t, journal.events, 1)
	assert.Equal(t, first.Pet.XP(), second.Pet.XP())
}

func TestStateViewDoesNotDoubleAwardUnderConcurrentReads(t *testing.T) {
	t.Parallel()

	journal := newFakeJournal()
	service := app.NewService(
		newFakeRepository(), journal, newFakeCache(), &serializingTx{}, fixedClock{now: testNow()},
	)
	userID := uuid.New()

	const readers = 8

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		applied int
	)

	wg.Add(readers)
	for range readers {
		go func() {
			defer wg.Done()

			view, err := service.StateView(context.Background(), userID)
			if err != nil {
				return
			}

			mu.Lock()
			defer mu.Unlock()

			if view.CheckInApplied {
				applied++
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, applied)
	assert.Len(t, journal.events, 1)
}

func TestStateViewKeepsExplicitCheckInEndpointWorking(t *testing.T) {
	t.Parallel()

	service, journal, _ := newTestService(t)
	userID := uuid.New()

	result, err := service.CheckIn(t.Context(), userID)
	require.NoError(t, err)
	require.Positive(t, result.Progress.XPGranted)

	view, err := service.StateView(t.Context(), userID)

	require.NoError(t, err)
	assert.False(t, view.CheckInApplied)
	assert.Len(t, journal.events, 1)
}
