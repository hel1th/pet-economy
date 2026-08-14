package app

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

type ActionCounter interface {
	CountByAction(ctx context.Context, userID uuid.UUID) (map[domain.Action]int, error)
}

type BadgeStatus struct {
	Badge   domain.Badge
	Current int
	Target  int
	Earned  bool
}

type BadgeService struct {
	badges  BadgeRepository
	pets    domain.Repository
	counter ActionCounter
	quests  *QuestService
	clock   Clock
}

type BadgeServiceDeps struct {
	Badges  BadgeRepository
	Pets    domain.Repository
	Counter ActionCounter
	Quests  *QuestService
	Clock   Clock
}

func NewBadgeService(deps BadgeServiceDeps) *BadgeService {
	return &BadgeService{
		badges: deps.Badges, pets: deps.Pets, counter: deps.Counter,
		quests: deps.Quests, clock: deps.Clock,
	}
}

func (s *BadgeService) Progress(ctx context.Context, userID uuid.UUID) ([]BadgeStatus, error) {
	catalog, err := s.badges.Catalog(ctx)
	if err != nil {
		return nil, err
	}

	stats, err := s.stats(ctx, userID)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]domain.BadgeProgress)
	for _, item := range domain.EvaluateBadges(stats) {
		byID[item.ID] = item
	}

	statuses := make([]BadgeStatus, 0, len(catalog))
	for _, badge := range catalog {
		progress := byID[badge.ID()]
		statuses = append(statuses, BadgeStatus{
			Badge:   badge,
			Current: progress.Current,
			Target:  progress.Target,
			Earned:  progress.Earned,
		})
	}

	return statuses, nil
}

func (s *BadgeService) AwardEarned(ctx context.Context, userID uuid.UUID) error {
	statuses, err := s.Progress(ctx, userID)
	if err != nil {
		return err
	}

	now := s.clock.Now()
	for _, status := range statuses {
		if !status.Earned {
			continue
		}
		if _, err := s.badges.Award(ctx, userID, status.Badge.ID(), now); err != nil {
			return err
		}
	}

	return nil
}

func (s *BadgeService) stats(ctx context.Context, userID uuid.UUID) (domain.BadgeStats, error) {
	counts, err := s.counter.CountByAction(ctx, userID)
	if err != nil {
		return domain.BadgeStats{}, err
	}

	stats := domain.BadgeStats{Actions: counts}

	pet, err := s.pets.ByUserID(ctx, userID)
	if err != nil && !errors.Is(err, domainerr.ErrNotFound) {
		return domain.BadgeStats{}, err
	}
	if pet != nil {
		stats.Level = pet.Level()
		stats.StreakDays = pet.StreakDays()
	}

	stats.QuestsDone, err = s.questsDone(ctx, userID)
	if err != nil {
		return domain.BadgeStats{}, err
	}

	return stats, nil
}

func (s *BadgeService) questsDone(ctx context.Context, userID uuid.UUID) (int, error) {
	if s.quests == nil {
		return 0, nil
	}

	views, err := s.quests.Today(ctx, userID)
	if err != nil {
		return 0, err
	}

	done := 0
	for _, view := range views {
		if view.Completed {
			done++
		}
	}

	return done, nil
}
