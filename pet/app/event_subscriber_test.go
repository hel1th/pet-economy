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
	"github.com/hel1th/pet-economy/shared/events"
)

type recordedXP struct {
	amount int
	reason string
	total  int
}

type spyNotifier struct {
	mu       sync.Mutex
	updated  []uuid.UUID
	xpGained []recordedXP
	levelUps []int
}

func (n *spyNotifier) PetUpdated(userID uuid.UUID, _ *domain.Pet) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.updated = append(n.updated, userID)
}

func (n *spyNotifier) XPGained(_ uuid.UUID, amount int, reason string, total int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.xpGained = append(n.xpGained, recordedXP{amount: amount, reason: reason, total: total})
}

func (n *spyNotifier) LevelUp(_ uuid.UUID, level int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.levelUps = append(n.levelUps, level)
}

func newSubscriberFixture(t *testing.T) (*app.Subscriber, *events.Bus, *spyNotifier, *fakeJournal) {
	t.Helper()

	journal := newFakeJournal()
	service := app.NewService(newFakeRepository(), journal, newFakeCache(), passthroughTx{}, fixedClock{now: testNow()})
	notifier := &spyNotifier{}
	subscriber := app.NewSubscriber(service, notifier, fixedClock{now: testNow()})

	bus := events.NewBus(nil)
	subscriber.Register(bus)

	return subscriber, bus, notifier, journal
}

func TestSubscriberRegisterIgnoresNilBus(t *testing.T) {
	t.Parallel()

	subscriber, _, _, _ := newSubscriberFixture(t)

	assert.NotPanics(t, func() { subscriber.Register(nil) })
}

func TestSubscriberAwardsOnItemPublished(t *testing.T) {
	t.Parallel()

	_, bus, notifier, journal := newSubscriberFixture(t)
	userID := uuid.New()

	bus.Publish(t.Context(), events.New(events.TypeItemPublished, userID, uuid.New(), testNow()))

	require.Len(t, notifier.updated, 1)
	assert.Equal(t, userID, notifier.updated[0])
	require.Len(t, notifier.xpGained, 1)
	assert.Positive(t, notifier.xpGained[0].amount)
	assert.Equal(t, string(events.TypeItemPublished), notifier.xpGained[0].reason)
	require.Len(t, journal.events, 1)
	assert.Equal(t, domain.ActionItemPublished, journal.events[0].Action)
}

func TestSubscriberAwardsOnItemSold(t *testing.T) {
	t.Parallel()

	_, bus, notifier, journal := newSubscriberFixture(t)
	userID := uuid.New()

	bus.Publish(t.Context(), events.New(events.TypeItemSold, userID, uuid.New(), testNow()))

	require.Len(t, journal.events, 1)
	assert.Equal(t, domain.ActionItemSold, journal.events[0].Action)
	require.Len(t, notifier.levelUps, 1)
	assert.Greater(t, notifier.levelUps[0], 1)
}

func TestSubscriberAwardsOnFavoriteAdded(t *testing.T) {
	t.Parallel()

	_, bus, notifier, journal := newSubscriberFixture(t)
	userID := uuid.New()

	bus.Publish(t.Context(), events.New(events.TypeFavoriteAdded, userID, uuid.New(), testNow()))

	require.Len(t, journal.events, 1)
	assert.Equal(t, domain.ActionFavorite, journal.events[0].Action)
	require.Len(t, notifier.xpGained, 1)
	assert.Equal(t, 1, notifier.xpGained[0].amount)
}

func TestSubscriberCreatesPetOnUserRegistered(t *testing.T) {
	t.Parallel()

	_, bus, notifier, journal := newSubscriberFixture(t)
	userID := uuid.New()

	bus.Publish(t.Context(), events.New(events.TypeUserRegistered, userID, uuid.Nil, testNow()))

	require.Len(t, notifier.updated, 1)
	assert.Equal(t, userID, notifier.updated[0])
	assert.Empty(t, notifier.xpGained)
	assert.Empty(t, notifier.levelUps)
	assert.Empty(t, journal.events)
}

func TestSubscriberSkipsDuplicateSubject(t *testing.T) {
	t.Parallel()

	_, bus, notifier, journal := newSubscriberFixture(t)
	userID, itemID := uuid.New(), uuid.New()

	event := events.New(events.TypeFavoriteAdded, userID, itemID, testNow())
	bus.Publish(t.Context(), event)
	bus.Publish(t.Context(), event)

	assert.Len(t, journal.events, 1)
	assert.Len(t, notifier.xpGained, 1)
}

func TestSubscriberSkipsWhenDailyLimitReached(t *testing.T) {
	t.Parallel()

	_, bus, notifier, journal := newSubscriberFixture(t)
	userID := uuid.New()

	for range 8 {
		bus.Publish(t.Context(), events.New(events.TypeFavoriteAdded, userID, uuid.New(), testNow()))
	}

	assert.Len(t, journal.events, 5)
	assert.Len(t, notifier.xpGained, 5)
}

func TestSubscriberPropagatesInfraErrors(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("journal is down")
	journal := &failingJournal{err: sentinel}
	service := app.NewService(newFakeRepository(), journal, newFakeCache(), passthroughTx{}, fixedClock{now: testNow()})
	notifier := &spyNotifier{}
	subscriber := app.NewSubscriber(service, notifier, fixedClock{now: testNow()})

	handled := make(map[events.Type]events.Handler)
	bus := events.NewBus(nil)
	subscriber.Register(recordingBus{bus: bus, seen: handled})

	err := handled[events.TypeItemPublished](t.Context(),
		events.New(events.TypeItemPublished, uuid.New(), uuid.New(), testNow()))

	require.ErrorIs(t, err, sentinel)
	assert.Empty(t, notifier.updated)
}

func TestSubscriberToleratesNilNotifier(t *testing.T) {
	t.Parallel()

	service := app.NewService(
		newFakeRepository(), newFakeJournal(), newFakeCache(), passthroughTx{}, fixedClock{now: testNow()})
	subscriber := app.NewSubscriber(service, nil, fixedClock{now: testNow()})

	bus := events.NewBus(nil)
	subscriber.Register(bus)

	assert.NotPanics(t, func() {
		bus.Publish(t.Context(), events.New(events.TypeItemSold, uuid.New(), uuid.New(), testNow()))
	})
}

type recordingBus struct {
	bus  *events.Bus
	seen map[events.Type]events.Handler
}

func (r recordingBus) Subscribe(t events.Type, h events.Handler) {
	r.seen[t] = h
	r.bus.Subscribe(t, h)
}

type failingJournal struct {
	err error
}

func (j *failingJournal) Append(context.Context, domain.XPEvent) error { return j.err }

func (j *failingJournal) CountSince(
	context.Context, uuid.UUID, domain.Action, time.Time,
) (int, error) {
	return 0, nil
}
