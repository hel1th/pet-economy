package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
)

type nudge struct {
	name     string
	hot      func(context.Context, HotState, time.Time) (HotState, bool, error)
	fallback func(*domain.Pet, time.Time)
}

func (s *Service) Stroke(ctx context.Context, userID uuid.UUID) (*domain.Pet, error) {
	action := nudge{name: "stroke", fallback: (*domain.Pet).Stroke}
	if s.hot != nil {
		action.hot = s.hot.Stroke
	}

	return s.applyNudge(ctx, userID, action)
}

func (s *Service) Feed(ctx context.Context, userID uuid.UUID) (*domain.Pet, error) {
	action := nudge{name: "feed", fallback: (*domain.Pet).FeedMeal}
	if s.hot != nil {
		if feeder, ok := s.hot.(HotFeeder); ok {
			action.hot = feeder.Feed
		}
	}

	return s.applyNudge(ctx, userID, action)
}

func (s *Service) applyNudge(
	ctx context.Context,
	userID uuid.UUID,
	action nudge,
) (*domain.Pet, error) {
	if action.hot == nil {
		return s.nudgeInDatabase(ctx, userID, action)
	}

	pet, err := s.readPet(ctx, userID)
	if err != nil {
		return s.nudgeInDatabase(ctx, userID, action)
	}
	pet.ApplyDecay(s.clock.Now())

	hot, _, err := action.hot(ctx, hotFromPet(pet), s.clock.Now())
	if err != nil {
		slog.WarnContext(ctx, "pet action via hot state failed, falling back to database",
			slog.String("action", action.name),
			slog.String("user_id", userID.String()), slog.Any("error", err))

		return s.nudgeInDatabase(ctx, userID, action)
	}
	if err := pet.ApplyHotState(hot.Happiness, hot.Satiety, hot.Version, hot.UpdatedAt); err != nil {
		return s.nudgeInDatabase(ctx, userID, action)
	}

	return pet, nil
}

func (s *Service) nudgeInDatabase(
	ctx context.Context,
	userID uuid.UUID,
	action nudge,
) (*domain.Pet, error) {
	return s.hatching(ctx, userID, func(_ context.Context, pet *domain.Pet) error {
		pet.ApplyDecay(s.clock.Now())
		action.fallback(pet, s.clock.Now())

		return nil
	})
}
