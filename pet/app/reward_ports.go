package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
)

type RewardRepository interface {
	Catalog(ctx context.Context) ([]domain.Reward, error)
	ByID(ctx context.Context, rewardID string) (domain.Reward, error)
	Grant(ctx context.Context, granted domain.UserReward) (bool, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.UserReward, error)
	ByUserAndReward(ctx context.Context, userID uuid.UUID, rewardID string) (domain.UserReward, error)
	Activate(ctx context.Context, granted domain.UserReward) (bool, error)
}

type BadgeRepository interface {
	Catalog(ctx context.Context) ([]domain.Badge, error)
	EarnedBy(ctx context.Context, userID uuid.UUID) ([]domain.EarnedBadge, error)
	Award(ctx context.Context, userID uuid.UUID, badgeID string, earnedAt time.Time) (bool, error)
}

type CodeSigner interface {
	Issue(userID uuid.UUID, rewardID string) (string, error)
	Verify(userID uuid.UUID, rewardID, code string) error
}

type RewardNotifier interface {
	RewardGranted(userID uuid.UUID, rewardID, title string)
}
