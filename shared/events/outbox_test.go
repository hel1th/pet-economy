package events_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/shared/events"
)

type recordingPublisher struct {
	mu       sync.Mutex
	received []events.Event
}

func (p *recordingPublisher) Publish(_ context.Context, e events.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.received = append(p.received, e)
}

func (p *recordingPublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.received)
}

type okTx struct{}

func (okTx) WithTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type rollbackTx struct{ err error }

func (f rollbackTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}

	return f.err
}

func sampleEvent() events.Event {
	return events.New(events.TypeItemPublished, uuid.New(), uuid.New(), time.Now().UTC())
}

func TestOutboxDrainReturnsAndClears(t *testing.T) {
	t.Parallel()

	outbox := events.NewOutbox()
	outbox.Add(sampleEvent())
	outbox.Add(sampleEvent())

	assert.Len(t, outbox.Drain(), 2)
	assert.Empty(t, outbox.Drain())
}

func TestOutboxFlushIgnoresNilPublisher(t *testing.T) {
	t.Parallel()

	outbox := events.NewOutbox()
	outbox.Add(sampleEvent())

	outbox.Flush(context.Background(), nil)

	assert.Len(t, outbox.Drain(), 1)
}

func TestPublishAfterCommitDeliversAllEvents(t *testing.T) {
	t.Parallel()

	publisher := &recordingPublisher{}

	err := events.PublishAfterCommit(t.Context(), publisher, okTx{}, func(_ context.Context, out *events.Outbox) error {
		out.Add(sampleEvent())
		out.Add(sampleEvent())

		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, publisher.count())
}

func TestPublishAfterCommitSkipsOnCallbackError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("business rule violated")
	publisher := &recordingPublisher{}

	err := events.PublishAfterCommit(t.Context(), publisher, okTx{}, func(_ context.Context, out *events.Outbox) error {
		out.Add(sampleEvent())

		return sentinel
	})

	require.ErrorIs(t, err, sentinel)
	assert.Zero(t, publisher.count())
}

func TestPublishAfterCommitRollbackDiscardsOutbox(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("commit failed")
	publisher := &recordingPublisher{}

	err := events.PublishAfterCommit(t.Context(), publisher, rollbackTx{err: sentinel},
		func(_ context.Context, out *events.Outbox) error {
			out.Add(sampleEvent())

			return nil
		})

	require.ErrorIs(t, err, sentinel)
	assert.Zero(t, publisher.count())
}

func TestOutboxIsConcurrencySafe(t *testing.T) {
	t.Parallel()

	outbox := events.NewOutbox()

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			outbox.Add(sampleEvent())
		}()
	}
	wg.Wait()

	assert.Len(t, outbox.Drain(), 50)
}

func TestEventPayloadAttribute(t *testing.T) {
	t.Parallel()

	event := sampleEvent().WithPayload(events.Payload{
		Title:      "Chair",
		Attributes: map[string]string{"color": "red"},
	})

	value, ok := event.Payload.Attribute("color")
	require.True(t, ok)
	assert.Equal(t, "red", value)

	_, missing := event.Payload.Attribute("size")
	assert.False(t, missing)
	assert.Equal(t, "Chair", event.Payload.Title)
}

func TestNopPublisherDoesNothing(t *testing.T) {
	t.Parallel()

	assert.NotPanics(t, func() {
		events.NopPublisher{}.Publish(context.Background(), sampleEvent())
	})
}
