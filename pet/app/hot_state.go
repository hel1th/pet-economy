package app

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type HotState struct {
	UserID    uuid.UUID
	Happiness int
	Satiety   int
	Version   int64
	UpdatedAt time.Time
}

type HotStateStore interface {
	GetOrInitialize(ctx context.Context, initial HotState) (HotState, error)
	Stroke(ctx context.Context, initial HotState, now time.Time) (HotState, bool, error)
	DirtyBatch(ctx context.Context, limit int64) ([]HotState, error)
	Acknowledge(ctx context.Context, state HotState) error
}

type HotFeeder interface {
	Feed(ctx context.Context, initial HotState, now time.Time) (HotState, bool, error)
}

type FeedCooldownReader interface {
	FeedAvailableAt(ctx context.Context, userID uuid.UUID, now time.Time) (*time.Time, error)
}

type HotStateWriter interface {
	SaveHotStates(ctx context.Context, states []HotState) error
}
