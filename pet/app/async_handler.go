package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/shared/events"
)

type EventDeduplicator interface {
	Claim(ctx context.Context, id uuid.UUID, e events.Event) (bool, error)
}

type AsyncHandler struct {
	subscriber *Subscriber
	dedup      EventDeduplicator
	tx         TxManager
	log        *slog.Logger
}

func NewAsyncHandler(
	subscriber *Subscriber,
	dedup EventDeduplicator,
	tx TxManager,
	log *slog.Logger,
) *AsyncHandler {
	if log == nil {
		log = slog.Default()
	}

	return &AsyncHandler{subscriber: subscriber, dedup: dedup, tx: tx, log: log}
}

func (h *AsyncHandler) Handle(ctx context.Context, e events.Event, id string) error {
	eventID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("parse event id: %w", err)
	}

	claimed, err := h.claim(ctx, eventID, e)
	if err != nil {
		return err
	}
	if !claimed {
		h.log.DebugContext(ctx, "skip already processed event",
			slog.String("event", string(e.Type)),
			slog.String("event_id", id),
		)

		return nil
	}

	return h.subscriber.Dispatch(ctx, e)
}

func (h *AsyncHandler) claim(ctx context.Context, eventID uuid.UUID, e events.Event) (bool, error) {
	if h.dedup == nil {
		return true, nil
	}

	var claimed bool
	err := h.tx.WithTx(ctx, func(ctx context.Context) error {
		var claimErr error
		claimed, claimErr = h.dedup.Claim(ctx, eventID, e)

		return claimErr
	})
	if err != nil {
		return false, fmt.Errorf("claim event: %w", err)
	}

	return claimed, nil
}
