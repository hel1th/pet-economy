package app

import (
	"context"
	"fmt"
	"log/slog"
)

const defaultFlushBatchSize = 100

type FlushHotStateHandler struct {
	pets      HotStateWriter
	cache     Cache
	hot       HotStateStore
	tx        TxManager
	log       *slog.Logger
	batchSize int64
}

func NewFlushHotStateHandler(
	pets HotStateWriter,
	cache Cache,
	hot HotStateStore,
	tx TxManager,
	batchSize int64,
	log *slog.Logger,
) *FlushHotStateHandler {
	if batchSize <= 0 {
		batchSize = defaultFlushBatchSize
	}
	if log == nil {
		log = slog.Default()
	}

	return &FlushHotStateHandler{
		pets: pets, cache: cache, hot: hot, tx: tx, batchSize: batchSize, log: log,
	}
}

func (h *FlushHotStateHandler) Handle(ctx context.Context) error {
	states, err := h.hot.DirtyBatch(ctx, h.batchSize)
	if err != nil {
		return fmt.Errorf("load dirty pet states: %w", err)
	}
	if len(states) == 0 {
		return nil
	}

	if err := h.tx.WithTx(ctx, func(ctx context.Context) error {
		return h.pets.SaveHotStates(ctx, states)
	}); err != nil {
		return fmt.Errorf("save hot pet states: %w", err)
	}

	for _, state := range states {
		if err := h.hot.Acknowledge(ctx, state); err != nil {
			h.log.WarnContext(ctx, "acknowledge hot pet state",
				slog.String("user_id", state.UserID.String()), slog.Any("error", err))

			continue
		}
		if err := h.cache.Delete(ctx, state.UserID); err != nil {
			h.log.DebugContext(ctx, "invalidate pet cache",
				slog.String("user_id", state.UserID.String()), slog.Any("error", err))
		}
	}

	return nil
}
