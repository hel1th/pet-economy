package events_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/shared/events"
)

func TestBus_DeliversToSubscribersOfType(t *testing.T) {
	t.Parallel()

	bus := events.NewBus(nil)
	userID := uuid.New()

	var published, sold int

	bus.Subscribe(events.TypeItemPublished, func(context.Context, events.Event) error {
		published++

		return nil
	})
	bus.Subscribe(events.TypeItemSold, func(context.Context, events.Event) error {
		sold++

		return nil
	})

	bus.Publish(context.Background(), events.New(events.TypeItemPublished, userID, uuid.New(), time.Now()))

	require.Equal(t, 1, published)
	require.Equal(t, 0, sold)
}

func TestBus_HandlerErrorDoesNotStopOthers(t *testing.T) {
	t.Parallel()

	bus := events.NewBus(nil)
	called := 0

	bus.Subscribe(events.TypeFavoriteAdded, func(context.Context, events.Event) error {
		return errors.New("boom")
	})
	bus.Subscribe(events.TypeFavoriteAdded, func(context.Context, events.Event) error {
		called++

		return nil
	})

	bus.Publish(context.Background(), events.New(events.TypeFavoriteAdded, uuid.New(), uuid.New(), time.Now()))

	require.Equal(t, 1, called)
}

func TestBus_PanicInHandlerDoesNotEscape(t *testing.T) {
	t.Parallel()

	bus := events.NewBus(nil)
	called := 0

	bus.Subscribe(events.TypeItemPublished, func(context.Context, events.Event) error {
		panic("subscriber exploded")
	})
	bus.Subscribe(events.TypeItemPublished, func(context.Context, events.Event) error {
		called++

		return nil
	})

	require.NotPanics(t, func() {
		bus.Publish(context.Background(), events.New(events.TypeItemPublished, uuid.New(), uuid.New(), time.Now()))
	})

	require.Equal(t, 1, called)
}

func TestBus_PanicIsReportedAsHandlerPanic(t *testing.T) {
	t.Parallel()

	bus := events.NewBus(nil)

	bus.Subscribe(events.TypeItemSold, func(context.Context, events.Event) error {
		panic(errors.New("nil map write"))
	})

	require.NotPanics(t, func() {
		bus.Publish(context.Background(), events.New(events.TypeItemSold, uuid.New(), uuid.New(), time.Now()))
	})
}

type failingTx struct{ fail bool }

func (f failingTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}
	if f.fail {
		return errors.New("commit failed")
	}

	return nil
}

func TestPublishAfterCommit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fail      bool
		wantCalls int
	}{
		{name: "commit delivers events", fail: false, wantCalls: 1},
		{name: "rollback swallows events", fail: true, wantCalls: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bus := events.NewBus(nil)
			calls := 0
			bus.Subscribe(events.TypeItemSold, func(context.Context, events.Event) error {
				calls++

				return nil
			})

			err := events.PublishAfterCommit(context.Background(), bus, failingTx{fail: tt.fail},
				func(_ context.Context, out *events.Outbox) error {
					out.Add(events.New(events.TypeItemSold, uuid.New(), uuid.New(), time.Now()))

					return nil
				})

			if tt.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tt.wantCalls, calls)
		})
	}
}
