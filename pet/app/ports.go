package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
)

type TxManager interface {
	WithTx(ctx context.Context, fn func(context.Context) error) error
}

type Clock interface {
	Now() time.Time
}

type Cache interface {
	Get(ctx context.Context, userID uuid.UUID) (*domain.Pet, error)
	Set(ctx context.Context, pet *domain.Pet) error
	Delete(ctx context.Context, userID uuid.UUID) error
}

type AccountGate interface {
	IsActive(ctx context.Context, userID uuid.UUID) (bool, error)
}

type XPJournal interface {
	Append(ctx context.Context, event domain.XPEvent) error
	CountSince(ctx context.Context, userID uuid.UUID, action domain.Action, since time.Time) (int, error)
}
