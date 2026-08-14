package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Action string

const (
	ActionDailyCheckIn   Action = "daily_checkin"
	ActionFavorite       Action = "favorite"
	ActionQualityListing Action = "quality_listing"
	ActionItemViewed     Action = "item_viewed"
)

type XPEvent struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Action    Action
	SubjectID *uuid.UUID
	Amount    int
	CreatedAt time.Time
}

func NewXPEvent(userID uuid.UUID, action Action, subjectID *uuid.UUID, amount int, now time.Time) XPEvent {
	return XPEvent{
		ID:        uuid.New(),
		UserID:    userID,
		Action:    action,
		SubjectID: subjectID,
		Amount:    amount,
		CreatedAt: now,
	}
}

type XPEventRepository interface {
	Append(ctx context.Context, event XPEvent) error
	CountSince(ctx context.Context, userID uuid.UUID, action Action, since time.Time) (int, error)
}

func DayStart(now time.Time) time.Time {
	return day(now)
}

func WeekStart(now time.Time) time.Time {
	start := day(now)
	offset := (int(start.Weekday()) + 6) % 7

	return start.AddDate(0, 0, -offset)
}
