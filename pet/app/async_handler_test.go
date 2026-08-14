package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/shared/events"
)

type fakeDeduplicator struct {
	mu     sync.Mutex
	seen   map[uuid.UUID]struct{}
	err    error
	claims int
}

func newFakeDeduplicator() *fakeDeduplicator {
	return &fakeDeduplicator{seen: make(map[uuid.UUID]struct{})}
}

func (d *fakeDeduplicator) Claim(_ context.Context, id uuid.UUID, _ events.Event) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.claims++
	if d.err != nil {
		return false, d.err
	}
	if _, ok := d.seen[id]; ok {
		return false, nil
	}
	d.seen[id] = struct{}{}

	return true, nil
}

func newAsyncFixture(t *testing.T) (*app.AsyncHandler, *fakeDeduplicator, *spyNotifier, *fakeJournal) {
	t.Helper()

	subscriber, _, notifier, journal := newSubscriberFixture(t)
	dedup := newFakeDeduplicator()

	return app.NewAsyncHandler(subscriber, dedup, passthroughTx{}, nil), dedup, notifier, journal
}

func TestAsyncHandlerAwardsOnFirstDelivery(t *testing.T) {
	t.Parallel()

	handler, _, notifier, journal := newAsyncFixture(t)
	userID := uuid.New()
	event := events.New(events.TypeItemPublished, userID, uuid.New(), testNow())

	require.NoError(t, handler.Handle(context.Background(), event, uuid.New().String()))

	assert.Len(t, notifier.xpGained, 1)
	assert.Len(t, journal.events, 1)
}

func TestAsyncHandlerIsIdempotentOnRedelivery(t *testing.T) {
	t.Parallel()

	handler, dedup, notifier, journal := newAsyncFixture(t)
	userID := uuid.New()
	event := events.New(events.TypeItemPublished, userID, uuid.New(), testNow())
	eventID := uuid.New().String()

	require.NoError(t, handler.Handle(context.Background(), event, eventID))
	require.NoError(t, handler.Handle(context.Background(), event, eventID))
	require.NoError(t, handler.Handle(context.Background(), event, eventID))

	assert.Equal(t, 3, dedup.claims, "every delivery must attempt a claim")
	assert.Len(t, notifier.xpGained, 1, "XP must be granted exactly once")
	assert.Len(t, journal.events, 1, "journal must record exactly one award")
}

func TestAsyncHandlerDistinctEventsBothAward(t *testing.T) {
	t.Parallel()

	handler, _, notifier, _ := newAsyncFixture(t)
	userID := uuid.New()

	first := events.New(events.TypeItemPublished, userID, uuid.New(), testNow())
	second := events.New(events.TypeItemPublished, userID, uuid.New(), testNow())

	require.NoError(t, handler.Handle(context.Background(), first, uuid.New().String()))
	require.NoError(t, handler.Handle(context.Background(), second, uuid.New().String()))

	assert.Len(t, notifier.xpGained, 2)
}

func TestAsyncHandlerRejectsMalformedEventID(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := newAsyncFixture(t)
	event := events.New(events.TypeItemPublished, uuid.New(), uuid.New(), testNow())

	require.Error(t, handler.Handle(context.Background(), event, "not-a-uuid"))
}

func TestAsyncHandlerPropagatesClaimFailure(t *testing.T) {
	t.Parallel()

	handler, dedup, _, journal := newAsyncFixture(t)
	dedup.err = errors.New("database unavailable")
	event := events.New(events.TypeItemPublished, uuid.New(), uuid.New(), testNow())

	err := handler.Handle(context.Background(), event, uuid.New().String())

	require.Error(t, err, "claim failure must be retryable, not silently dropped")
	assert.Empty(t, journal.events)
}

func TestAsyncHandlerIgnoresUnknownEventType(t *testing.T) {
	t.Parallel()

	handler, _, notifier, _ := newAsyncFixture(t)
	event := events.New(events.Type("unknown.event"), uuid.New(), uuid.New(), testNow())

	require.NoError(t, handler.Handle(context.Background(), event, uuid.New().String()))
	assert.Empty(t, notifier.xpGained)
}
