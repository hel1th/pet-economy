package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

type subjectChecker interface {
	ExistsBySubject(ctx context.Context, userID uuid.UUID, action domain.Action, subjectID uuid.UUID) (bool, error)
}

type AwardCommand struct {
	UserID    uuid.UUID
	Action    domain.Action
	SubjectID uuid.UUID
	Apply     func(*domain.Pet, domain.LimitedAction) error
}

func (s *Service) AwardAction(ctx context.Context, cmd AwardCommand) (*domain.Pet, error) {
	now := s.clock.Now()

	active, err := s.accountActive(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, domainerr.ErrNotFound
	}

	return s.hatching(ctx, cmd.UserID, func(ctx context.Context, pet *domain.Pet) error {
		rewarded, err := s.journal.CountSince(ctx, cmd.UserID, cmd.Action, domain.DayStart(now))
		if err != nil {
			return fmt.Errorf("count rewarded actions: %w", err)
		}

		unique, err := s.isUniqueSubject(ctx, cmd)
		if err != nil {
			return err
		}

		before := pet.XP()

		applyErr := cmd.Apply(pet, domain.LimitedAction{
			SubjectID:     cmd.SubjectID,
			Unique:        unique,
			RewardedCount: rewarded,
			Now:           now,
		})
		if applyErr != nil {
			return applyErr
		}

		granted := pet.XP() - before
		if granted <= 0 {
			return nil
		}

		subjectID := cmd.SubjectID

		return s.journal.Append(ctx, domain.NewXPEvent(cmd.UserID, cmd.Action, &subjectID, granted, now))
	})
}

func (s *Service) accountActive(ctx context.Context, userID uuid.UUID) (bool, error) {
	if s.accounts == nil {
		return true, nil
	}

	active, err := s.accounts.IsActive(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("check account state: %w", err)
	}

	return active, nil
}

func (s *Service) isUniqueSubject(ctx context.Context, cmd AwardCommand) (bool, error) {
	checker, ok := s.journal.(subjectChecker)
	if !ok {
		return true, nil
	}

	seen, err := checker.ExistsBySubject(ctx, cmd.UserID, cmd.Action, cmd.SubjectID)
	if err != nil {
		return false, fmt.Errorf("check action uniqueness: %w", err)
	}

	return !seen, nil
}
