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
)

type cooldownHotStore struct {
	configurableHotStore

	availableAt *time.Time
	applied     bool
	ttlErr      error
	ttlCalls    int
}

func (s *cooldownHotStore) FeedAvailableAt(
	_ context.Context,
	_ uuid.UUID,
	_ time.Time,
) (*time.Time, error) {
	s.ttlCalls++
	if s.ttlErr != nil {
		return nil, s.ttlErr
	}

	return s.availableAt, nil
}

func (s *cooldownHotStore) Feed(
	_ context.Context,
	initial app.HotState,
	now time.Time,
) (app.HotState, bool, error) {
	s.feeds++
	if !s.applied {
		return initial, false, nil
	}

	initial.Satiety = min(initial.Satiety+15, 100)
	initial.Version++
	initial.UpdatedAt = now

	return initial, true, nil
}

func TestStateViewReportsFeedAvailability(t *testing.T) {
	t.Parallel()

	available := testNow().Add(5 * time.Hour)
	hot := &cooldownHotStore{availableAt: &available}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)

	view, err := service.StateView(t.Context(), uuid.New())

	require.NoError(t, err)
	require.NotNil(t, view.FeedAvailableAt)
	assert.Equal(t, available, *view.FeedAvailableAt)
	assert.Equal(t, 1, hot.ttlCalls)
}

func TestStateViewOmitsFeedAvailabilityWhenCooldownExpired(t *testing.T) {
	t.Parallel()

	hot := &cooldownHotStore{}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)

	view, err := service.StateView(t.Context(), uuid.New())

	require.NoError(t, err)
	assert.Nil(t, view.FeedAvailableAt)
}

func TestStateViewSurvivesCooldownReadFailure(t *testing.T) {
	t.Parallel()

	hot := &cooldownHotStore{ttlErr: errors.New("redis down")}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)

	view, err := service.StateView(t.Context(), uuid.New())

	require.NoError(t, err)
	assert.Nil(t, view.FeedAvailableAt)
	require.NotNil(t, view.Pet)
}

func TestStateViewWithoutHotStateHasNoFeedDeadline(t *testing.T) {
	t.Parallel()

	service, _, _ := newTestService(t)

	view, err := service.StateView(t.Context(), uuid.New())

	require.NoError(t, err)
	assert.Nil(t, view.FeedAvailableAt)
}

func TestFeedViewReportsNextFeedTime(t *testing.T) {
	t.Parallel()

	available := testNow().Add(5 * time.Hour)
	hot := &cooldownHotStore{availableAt: &available, applied: true}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)
	userID := uuid.New()

	created, err := service.Create(t.Context(), userID)
	require.NoError(t, err)
	before := created.Satiety()

	view, err := service.FeedView(t.Context(), userID)

	require.NoError(t, err)
	require.NotNil(t, view.Pet)
	require.NotNil(t, view.FeedAvailableAt)
	assert.Equal(t, available, *view.FeedAvailableAt)
	assert.False(t, view.CheckInApplied)
	assert.Equal(t, 1, hot.feeds)
	assert.Greater(t, view.Pet.Satiety(), before)
}

func TestFeedViewWithinCooldownKeepsSatiety(t *testing.T) {
	t.Parallel()

	available := testNow().Add(2 * time.Hour)
	hot := &cooldownHotStore{availableAt: &available}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)
	userID := uuid.New()

	created, err := service.Create(t.Context(), userID)
	require.NoError(t, err)
	before := created.Satiety()

	view, err := service.FeedView(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, before, view.Pet.Satiety())
	require.NotNil(t, view.FeedAvailableAt)
	assert.Equal(t, available, *view.FeedAvailableAt)
}

func TestFeedViewClearsDeadlineOnceCooldownElapsed(t *testing.T) {
	t.Parallel()

	hot := &cooldownHotStore{applied: true}
	service, _, _ := newTestService(t)
	service = service.WithHotState(hot)
	userID := uuid.New()

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	view, err := service.FeedView(t.Context(), userID)

	require.NoError(t, err)
	assert.Nil(t, view.FeedAvailableAt)
}
