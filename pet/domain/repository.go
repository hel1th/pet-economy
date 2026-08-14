package domain

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	ByUserID(ctx context.Context, userID uuid.UUID) (*Pet, error)
	ByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (*Pet, error)
	Save(ctx context.Context, pet *Pet) error
}
