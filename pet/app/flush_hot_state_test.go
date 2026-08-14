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
)

type fakeHotStore struct {
	mu           sync.Mutex
	dirty        []app.HotState
	acknowledged []app.HotState
	batchErr     error
	ackErr       error
}

func (s *fakeHotStore) GetOrInitialize(_ context.Context, initial app.HotState) (app.HotState, error) {
	return initial, nil
}

func (s *fakeHotStore) Stroke(_ context.Context, initial app.HotState, now time.Time) (app.HotState, bool, error) {
	initial.Happiness = min(initial.Happiness+5, 100)
	initial.Version++
	initial.UpdatedAt = now

	return initial, true, nil
}

func (s *fakeHotStore) DirtyBatch(_ context.Context, limit int64) ([]app.HotState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.batchErr != nil {
		return nil, s.batchErr
	}
	if int64(len(s.dirty)) <= limit {
		return s.dirty, nil
	}

	return s.dirty[:limit], nil
}

func (s *fakeHotStore) Acknowledge(_ context.Context, state app.HotState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ackErr != nil {
		return s.ackErr
	}
	s.acknowledged = append(s.acknowledged, state)

	return nil
}

type fakeHotWriter struct {
	mu      sync.Mutex
	saved   [][]app.HotState
	saveErr error
}

func (w *fakeHotWriter) SaveHotStates(_ context.Context, states []app.HotState) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.saveErr != nil {
		return w.saveErr
	}
	w.saved = append(w.saved, states)

	return nil
}

func sampleHotState() app.HotState {
	return app.HotState{
		UserID: uuid.New(), Happiness: 80, Satiety: 60, Version: 3, UpdatedAt: testNow(),
	}
}

func TestFlushPersistsAndAcknowledgesDirtyStates(t *testing.T) {
	t.Parallel()

	states := []app.HotState{sampleHotState(), sampleHotState()}
	hot := &fakeHotStore{dirty: states}
	writer := &fakeHotWriter{}
	cache := newFakeCache()

	handler := app.NewFlushHotStateHandler(writer, cache, hot, passthroughTx{}, 100, nil)
	require.NoError(t, handler.Handle(context.Background()))

	require.Len(t, writer.saved, 1)
	assert.Len(t, writer.saved[0], 2)
	assert.Len(t, hot.acknowledged, 2, "each persisted state must be acknowledged")
}

func TestFlushIsNoopWithoutDirtyStates(t *testing.T) {
	t.Parallel()

	hot := &fakeHotStore{}
	writer := &fakeHotWriter{}

	handler := app.NewFlushHotStateHandler(writer, newFakeCache(), hot, passthroughTx{}, 100, nil)
	require.NoError(t, handler.Handle(context.Background()))

	assert.Empty(t, writer.saved)
	assert.Empty(t, hot.acknowledged)
}

func TestFlushDoesNotAcknowledgeWhenPersistFails(t *testing.T) {
	t.Parallel()

	hot := &fakeHotStore{dirty: []app.HotState{sampleHotState()}}
	writer := &fakeHotWriter{saveErr: errors.New("database unavailable")}

	handler := app.NewFlushHotStateHandler(writer, newFakeCache(), hot, passthroughTx{}, 100, nil)

	require.Error(t, handler.Handle(context.Background()))
	assert.Empty(t, hot.acknowledged, "unpersisted state must stay dirty for the next flush")
}

func TestFlushRespectsBatchSize(t *testing.T) {
	t.Parallel()

	hot := &fakeHotStore{dirty: []app.HotState{sampleHotState(), sampleHotState(), sampleHotState()}}
	writer := &fakeHotWriter{}

	handler := app.NewFlushHotStateHandler(writer, newFakeCache(), hot, passthroughTx{}, 2, nil)
	require.NoError(t, handler.Handle(context.Background()))

	require.Len(t, writer.saved, 1)
	assert.Len(t, writer.saved[0], 2)
}

func TestFlushSurvivesAcknowledgeFailure(t *testing.T) {
	t.Parallel()

	hot := &fakeHotStore{
		dirty:  []app.HotState{sampleHotState()},
		ackErr: errors.New("redis unavailable"),
	}
	writer := &fakeHotWriter{}

	handler := app.NewFlushHotStateHandler(writer, newFakeCache(), hot, passthroughTx{}, 100, nil)

	require.NoError(t, handler.Handle(context.Background()), "data is already durable in postgres")
	assert.Len(t, writer.saved, 1)
}

func TestFlushReportsDirtyBatchFailure(t *testing.T) {
	t.Parallel()

	hot := &fakeHotStore{batchErr: errors.New("redis unavailable")}
	writer := &fakeHotWriter{}

	handler := app.NewFlushHotStateHandler(writer, newFakeCache(), hot, passthroughTx{}, 100, nil)

	require.Error(t, handler.Handle(context.Background()))
	assert.Empty(t, writer.saved)
}
