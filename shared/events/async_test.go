package events_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/shared/events"
)

type stubTransport struct {
	mu       sync.Mutex
	err      error
	received []events.Event
}

func (t *stubTransport) Publish(_ context.Context, e events.Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.err != nil {
		return t.err
	}
	t.received = append(t.received, e)

	return nil
}

func (t *stubTransport) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.received)
}

func (t *stubTransport) setErr(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.err = err
}

func TestAsyncPublisherUsesTransportWhenHealthy(t *testing.T) {
	t.Parallel()

	transport := &stubTransport{}
	fallback := &recordingPublisher{}

	publisher := events.NewAsyncPublisher(transport, fallback, nil)
	publisher.Publish(context.Background(), sampleEvent())

	assert.Equal(t, 1, transport.count())
	assert.Zero(t, fallback.count(), "fallback must not double-deliver")
	assert.False(t, publisher.Degraded())
}

func TestAsyncPublisherFallsBackWhenBrokerDown(t *testing.T) {
	t.Parallel()

	transport := &stubTransport{err: errors.New("broker unavailable")}
	fallback := &recordingPublisher{}

	publisher := events.NewAsyncPublisher(transport, fallback, nil)
	publisher.Publish(context.Background(), sampleEvent())

	assert.Zero(t, transport.count())
	assert.Equal(t, 1, fallback.count(), "event must still be delivered in-process")
	assert.True(t, publisher.Degraded())
}

func TestAsyncPublisherRecoversAfterOutage(t *testing.T) {
	t.Parallel()

	transport := &stubTransport{err: errors.New("broker unavailable")}
	fallback := &recordingPublisher{}

	publisher := events.NewAsyncPublisher(transport, fallback, nil)
	publisher.Publish(context.Background(), sampleEvent())
	require.True(t, publisher.Degraded())

	transport.setErr(nil)
	publisher.Publish(context.Background(), sampleEvent())

	assert.False(t, publisher.Degraded())
	assert.Equal(t, 1, transport.count())
	assert.Equal(t, 1, fallback.count(), "only the outage event went through the fallback")
}

func TestAsyncPublisherWithoutTransportUsesFallback(t *testing.T) {
	t.Parallel()

	fallback := &recordingPublisher{}

	publisher := events.NewAsyncPublisher(nil, fallback, nil)
	publisher.Publish(context.Background(), sampleEvent())

	assert.Equal(t, 1, fallback.count())
}

func TestAsyncPublisherWithoutFallbackDoesNotPanic(t *testing.T) {
	t.Parallel()

	transport := &stubTransport{err: errors.New("broker unavailable")}
	publisher := events.NewAsyncPublisher(transport, nil, nil)

	assert.NotPanics(t, func() { publisher.Publish(context.Background(), sampleEvent()) })
}
