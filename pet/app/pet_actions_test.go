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
	"github.com/hel1th/pet-economy/shared/domainerr"
)

func TestFavoriteRespectsDailyLimitFromJournal(t *testing.T) {
	t.Parallel()

	service, journal, _ := newTestService(t)
	userID := uuid.New()

	for range 5 {
		_, err := service.AddFavorite(t.Context(), userID, uuid.New())
		require.NoError(t, err)
	}

	_, err := service.AddFavorite(t.Context(), userID, uuid.New())
	require.ErrorIs(t, err, domain.ErrLimitReached)
	assert.Len(t, journal.events, 5)
}

func TestFavoriteIsIdempotentPerSubject(t *testing.T) {
	t.Parallel()

	service, journal, _ := newTestService(t)
	userID := uuid.New()
	itemID := uuid.New()

	_, err := service.AddFavorite(t.Context(), userID, itemID)
	require.NoError(t, err)

	_, err = service.AddFavorite(t.Context(), userID, itemID)
	require.ErrorIs(t, err, domain.ErrDuplicateAction)
	assert.Len(t, journal.events, 1)
}

func TestItemViewsUseDailyLimit(t *testing.T) {
	t.Parallel()

	service, _, _ := newTestService(t)
	userID := uuid.New()

	for range domain.MaxItemViewsPerDay {
		_, err := service.ViewItem(t.Context(), userID, uuid.New())
		require.NoError(t, err)
	}

	_, err := service.ViewItem(t.Context(), userID, uuid.New())
	require.ErrorIs(t, err, domain.ErrLimitReached)
}

func TestFailedActionInvalidatesCache(t *testing.T) {
	t.Parallel()

	service, _, cache := newTestService(t)
	userID := uuid.New()
	itemID := uuid.New()

	_, err := service.AddFavorite(t.Context(), userID, itemID)
	require.NoError(t, err)
	require.NotNil(t, cache.stored[userID])

	_, err = service.AddFavorite(t.Context(), userID, itemID)
	require.Error(t, err)
	assert.Nil(t, cache.stored[userID])
}

func TestStateAppliesDecayOnCacheHit(t *testing.T) {
	t.Parallel()

	clock := &movableClock{now: testNow()}
	repository := newFakeRepository()
	cache := newFakeCache()
	service := app.NewService(repository, newFakeJournal(), cache, passthroughTx{}, clock)
	userID := uuid.New()

	created, err := service.Create(t.Context(), userID)
	require.NoError(t, err)
	require.Equal(t, 70, created.Satiety())

	first, err := service.State(t.Context(), userID)
	require.NoError(t, err)
	require.Equal(t, 70, first.Satiety())

	clock.now = clock.now.Add(10 * time.Hour)
	second, err := service.State(t.Context(), userID)
	require.NoError(t, err)

	assert.Equal(t, 50, second.Satiety())
	assert.Equal(t, 2, cache.hits)
}

func newTestService(t *testing.T) (*app.Service, *fakeJournal, *fakeCache) {
	t.Helper()

	journal := newFakeJournal()
	cache := newFakeCache()
	service := app.NewService(newFakeRepository(), journal, cache, passthroughTx{}, fixedClock{now: testNow()})

	return service, journal, cache
}

func testNow() time.Time {
	return time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time { return c.now }

type movableClock struct {
	now time.Time
}

func (c *movableClock) Now() time.Time { return c.now }

type passthroughTx struct{}

func (passthroughTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type fakeRepository struct {
	pets map[uuid.UUID]*domain.Pet
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{pets: make(map[uuid.UUID]*domain.Pet)}
}

func (r *fakeRepository) ByUserID(_ context.Context, userID uuid.UUID) (*domain.Pet, error) {
	pet, ok := r.pets[userID]
	if !ok {
		return nil, domainerr.ErrNotFound
	}

	return pet, nil
}

func (r *fakeRepository) ByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (*domain.Pet, error) {
	return r.ByUserID(ctx, userID)
}

func (r *fakeRepository) Save(_ context.Context, pet *domain.Pet) error {
	r.pets[pet.UserID()] = pet

	return nil
}

type journalKey struct {
	userID  uuid.UUID
	action  domain.Action
	subject uuid.UUID
}

type fakeJournal struct {
	events []domain.XPEvent
	seen   map[journalKey]bool
}

func newFakeJournal() *fakeJournal {
	return &fakeJournal{seen: make(map[journalKey]bool)}
}

func (j *fakeJournal) Append(_ context.Context, event domain.XPEvent) error {
	if event.SubjectID != nil {
		key := journalKey{userID: event.UserID, action: event.Action, subject: *event.SubjectID}
		if j.seen[key] {
			return domain.ErrDuplicateAction
		}
		j.seen[key] = true
	}
	j.events = append(j.events, event)

	return nil
}

func (j *fakeJournal) CountSince(
	_ context.Context,
	userID uuid.UUID,
	action domain.Action,
	since time.Time,
) (int, error) {
	count := 0
	for _, event := range j.events {
		if event.UserID == userID && event.Action == action && !event.CreatedAt.Before(since) {
			count++
		}
	}

	return count, nil
}

type fakeCache struct {
	stored map[uuid.UUID]*domain.Pet
	hits   int
}

func newFakeCache() *fakeCache {
	return &fakeCache{stored: make(map[uuid.UUID]*domain.Pet)}
}

func (c *fakeCache) Get(_ context.Context, userID uuid.UUID) (*domain.Pet, error) {
	pet, ok := c.stored[userID]
	if !ok {
		return nil, nil
	}
	c.hits++

	return pet, nil
}

func (c *fakeCache) Set(_ context.Context, pet *domain.Pet) error {
	c.stored[pet.UserID()] = pet

	return nil
}

func (c *fakeCache) Delete(_ context.Context, userID uuid.UUID) error {
	delete(c.stored, userID)

	return nil
}
