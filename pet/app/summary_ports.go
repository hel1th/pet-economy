package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
)

type SummaryRepository interface {
	ByUserAndDate(ctx context.Context, userID uuid.UUID, date time.Time) (*domain.DailySummary, error)
	Insert(ctx context.Context, summary *domain.DailySummary) (bool, error)
	ListByUser(ctx context.Context, f SummaryHistoryFilter) ([]*domain.DailySummary, error)
}

type SummaryHistoryFilter struct {
	UserID uuid.UUID
	Before time.Time
	Limit  int
}

type XPAggregate struct {
	Action domain.Action
	Count  int
	Amount int
}

type DayActivityReadModel interface {
	XPByAction(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]XPAggregate, error)
	XPTotalBefore(ctx context.Context, userID uuid.UUID, before time.Time) (int, error)
	RewardsGranted(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]string, error)
	BadgesEarned(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]string, error)
	ListingIssues(ctx context.Context, userID uuid.UUID, limit int) ([]domain.ListingIssue, error)
}

type RankReadModel interface {
	RankOf(ctx context.Context, userID uuid.UUID) (int, bool, error)
}

type SummaryGenerator interface {
	Generate(ctx context.Context, facts domain.DayFacts) (message string, ok bool)
}
