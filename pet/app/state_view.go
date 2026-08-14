package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
)

type StateView struct {
	Pet             *domain.Pet
	FeedAvailableAt *time.Time
	CheckInApplied  bool
	Progress        domain.Progress
	Streak          domain.StreakOutcome
}

func (s *Service) StateView(ctx context.Context, userID uuid.UUID) (StateView, error) {
	view := StateView{}

	result, applied, err := s.autoCheckIn(ctx, userID)
	if err != nil {
		return StateView{}, err
	}

	if applied {
		view.CheckInApplied = true
		view.Progress = result.Progress
		view.Streak = result.Streak
	}

	pet, err := s.State(ctx, userID)
	if err != nil {
		return StateView{}, err
	}
	view.Pet = pet
	view.FeedAvailableAt = s.feedAvailableAt(ctx, userID)

	return view, nil
}

func (s *Service) FeedView(ctx context.Context, userID uuid.UUID) (StateView, error) {
	pet, err := s.Feed(ctx, userID)
	if err != nil {
		return StateView{}, err
	}

	return StateView{Pet: pet, FeedAvailableAt: s.feedAvailableAt(ctx, userID)}, nil
}

func (s *Service) autoCheckIn(ctx context.Context, userID uuid.UUID) (ActionResult, bool, error) {
	result, err := s.CheckIn(ctx, userID)
	if errors.Is(err, domain.ErrDuplicateAction) {
		return ActionResult{}, false, nil
	}
	if err != nil {
		return ActionResult{}, false, err
	}

	return result, true, nil
}

func (s *Service) feedAvailableAt(ctx context.Context, userID uuid.UUID) *time.Time {
	if s.hot == nil {
		return nil
	}
	reader, ok := s.hot.(FeedCooldownReader)
	if !ok {
		return nil
	}

	available, err := reader.FeedAvailableAt(ctx, userID, s.clock.Now())
	if err != nil {
		slog.WarnContext(ctx, "read feed cooldown",
			slog.String("user_id", userID.String()), slog.Any("error", err))

		return nil
	}

	return available
}
