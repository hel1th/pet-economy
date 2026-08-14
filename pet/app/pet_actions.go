package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
)

type ActionResult struct {
	Pet      *domain.Pet
	Progress domain.Progress
	Streak   domain.StreakOutcome
}

func (s *Service) CheckIn(ctx context.Context, userID uuid.UUID) (ActionResult, error) {
	now := s.clock.Now()
	result := ActionResult{}

	pet, err := s.hatching(ctx, userID, func(ctx context.Context, pet *domain.Pet) error {
		outcome, err := pet.CheckIn(now)
		if err != nil {
			return err
		}
		if err := s.record(ctx, userID, domain.ActionDailyCheckIn, nil, outcome.Progress.XPGranted, now); err != nil {
			return err
		}
		result.Progress = outcome.Progress
		result.Streak = outcome.Streak

		return nil
	})
	if err != nil {
		return ActionResult{}, err
	}
	result.Pet = pet

	return result, nil
}

func (s *Service) AddFavorite(ctx context.Context, userID, itemID uuid.UUID) (ActionResult, error) {
	return s.limited(ctx, userID, itemID, limitedRule{
		action: domain.ActionFavorite,
		limit:  domain.MaxFavoritesPerDay,
		since:  domain.DayStart,
		award: func(pet *domain.Pet, action domain.LimitedAction) (domain.Progress, error) {
			return pet.RewardFavorite(action)
		},
	})
}

func (s *Service) ViewItem(ctx context.Context, userID, itemID uuid.UUID) (ActionResult, error) {
	return s.limited(ctx, userID, itemID, limitedRule{
		action: domain.ActionItemViewed,
		limit:  domain.MaxItemViewsPerDay,
		since:  domain.DayStart,
		award: func(pet *domain.Pet, action domain.LimitedAction) (domain.Progress, error) {
			return pet.RewardItemViewed(action)
		},
	})
}

func (s *Service) RewardQualityListing(
	ctx context.Context,
	userID, itemID uuid.UUID,
	listing domain.QualityListing,
) (ActionResult, error) {
	now := s.clock.Now()
	listing.Now = now
	result := ActionResult{}

	pet, err := s.hatching(ctx, userID, func(ctx context.Context, pet *domain.Pet) error {
		progress, err := pet.RewardQualityListing(listing)
		if err != nil {
			return err
		}
		if err := s.record(ctx, userID, domain.ActionQualityListing, &itemID, progress.XPGranted, now); err != nil {
			return err
		}
		result.Progress = progress

		return nil
	})
	if err != nil {
		return ActionResult{}, err
	}
	result.Pet = pet

	return result, nil
}

type limitedRule struct {
	action domain.Action
	limit  int
	since  func(time.Time) time.Time
	award  func(*domain.Pet, domain.LimitedAction) (domain.Progress, error)
}

func (s *Service) limited(
	ctx context.Context,
	userID, subjectID uuid.UUID,
	rule limitedRule,
) (ActionResult, error) {
	now := s.clock.Now()
	result := ActionResult{}

	pet, err := s.hatching(ctx, userID, func(ctx context.Context, pet *domain.Pet) error {
		count, err := s.journal.CountSince(ctx, userID, rule.action, rule.since(now))
		if err != nil {
			return err
		}
		if count >= rule.limit {
			return domain.ErrLimitReached
		}

		unique, err := s.isUniqueSubject(ctx, AwardCommand{
			UserID: userID, Action: rule.action, SubjectID: subjectID,
		})
		if err != nil {
			return err
		}

		progress, err := rule.award(pet, domain.LimitedAction{
			SubjectID: subjectID, Unique: unique, RewardedCount: count, Now: now,
		})
		if err != nil {
			return err
		}
		if err := s.record(ctx, userID, rule.action, &subjectID, progress.XPGranted, now); err != nil {
			return err
		}
		result.Progress = progress

		return nil
	})
	if err != nil {
		return ActionResult{}, err
	}
	result.Pet = pet

	return result, nil
}

func (s *Service) record(
	ctx context.Context,
	userID uuid.UUID,
	action domain.Action,
	subjectID *uuid.UUID,
	amount int,
	now time.Time,
) error {
	return s.journal.Append(ctx, domain.NewXPEvent(userID, action, subjectID, amount, now))
}
