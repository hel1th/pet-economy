package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
	"github.com/hel1th/pet-economy/shared/events"
)

type PetNotifier interface {
	PetUpdated(userID uuid.UUID, pet *domain.Pet)
	XPGained(userID uuid.UUID, amount int, reason string, total int)
	LevelUp(userID uuid.UUID, level int)
}

type reaction struct {
	action domain.Action
	award  func(*domain.Pet, domain.LimitedAction) (domain.Progress, error)
}

type Subscriber struct {
	service  *Service
	notifier PetNotifier
	clock    Clock
	rewards  *RewardService
	badges   *BadgeService
}

func NewSubscriber(service *Service, notifier PetNotifier, clock Clock) *Subscriber {
	return &Subscriber{service: service, notifier: notifier, clock: clock}
}

func (s *Subscriber) WithRewards(rewards *RewardService) *Subscriber {
	s.rewards = rewards

	return s
}

func (s *Subscriber) WithBadges(badges *BadgeService) *Subscriber {
	s.badges = badges

	return s
}

func (s *Subscriber) routes() map[events.Type]events.Handler {
	return map[events.Type]events.Handler{
		events.TypeItemPublished:  s.onItemPublished,
		events.TypeItemSold:       s.onItemSold,
		events.TypeItemUpdated:    s.onItemUpdated,
		events.TypeFavoriteAdded:  s.onFavoriteAdded,
		events.TypeItemViewed:     s.onItemViewed,
		events.TypeUserRegistered: s.onUserRegistered,
	}
}

func (s *Subscriber) Register(bus events.Subscriber) {
	if bus == nil {
		return
	}

	for eventType, handler := range s.routes() {
		bus.Subscribe(eventType, handler)
	}
}

func (s *Subscriber) Dispatch(ctx context.Context, e events.Event) error {
	handler, ok := s.routes()[e.Type]
	if !ok {
		return nil
	}

	return handler(ctx, e)
}

func (s *Subscriber) onItemPublished(ctx context.Context, e events.Event) error {
	return s.award(ctx, e, reaction{
		action: domain.ActionItemPublished,
		award:  (*domain.Pet).RewardItemPublished,
	})
}

func (s *Subscriber) onItemSold(ctx context.Context, e events.Event) error {
	return s.award(ctx, e, reaction{
		action: domain.ActionItemSold,
		award:  (*domain.Pet).RewardItemSold,
	})
}

func (s *Subscriber) onItemUpdated(ctx context.Context, e events.Event) error {
	improvement := improvementFrom(e.Payload)
	if !improvement.Any() {
		return s.award(ctx, e, reaction{
			action: domain.ActionItemUpdated,
			award:  (*domain.Pet).RewardItemUpdated,
		})
	}

	return s.award(ctx, e, reaction{
		action: domain.ActionItemImproved,
		award: func(p *domain.Pet, a domain.LimitedAction) (domain.Progress, error) {
			return p.RewardItemImproved(a, improvement)
		},
	})
}

func (s *Subscriber) onItemViewed(ctx context.Context, e events.Event) error {
	return s.award(ctx, e, reaction{
		action: domain.ActionItemViewed,
		award:  (*domain.Pet).RewardItemViewed,
	})
}

func improvementFrom(payload events.Payload) domain.ListingImprovement {
	flag := func(key string) bool {
		value, ok := payload.Attribute(key)

		return ok && value == events.AttrFlagTrue
	}

	return domain.ListingImprovement{
		PhotoAdded:       flag(events.AttrPhotoAdded),
		DescriptionAdded: flag(events.AttrDescriptionAdded),
		PriceSet:         flag(events.AttrPriceSet),
	}
}

func (s *Subscriber) onFavoriteAdded(ctx context.Context, e events.Event) error {
	return s.award(ctx, e, reaction{
		action: domain.ActionFavorite,
		award: func(p *domain.Pet, a domain.LimitedAction) (domain.Progress, error) {
			return p.RewardFavorite(a)
		},
	})
}

func (s *Subscriber) onUserRegistered(ctx context.Context, e events.Event) error {
	pet, err := s.service.Create(ctx, e.UserID)
	if err != nil {
		return err
	}

	s.notify(e.UserID, pet, domain.Progress{}, string(events.TypeUserRegistered))

	return nil
}

func (s *Subscriber) award(ctx context.Context, e events.Event, r reaction) error {
	var progress domain.Progress

	pet, err := s.service.AwardAction(ctx, AwardCommand{
		UserID:    e.UserID,
		Action:    r.action,
		SubjectID: e.SubjectID,
		Apply: func(p *domain.Pet, a domain.LimitedAction) error {
			result, awardErr := r.award(p, a)
			if awardErr != nil {
				return awardErr
			}
			progress = result

			return nil
		},
	})
	if err != nil {
		if isSkippable(err) {
			return nil
		}

		return err
	}

	s.grantRewards(ctx, e.UserID, progress)
	s.grantBadges(ctx, e.UserID)
	s.notify(e.UserID, pet, progress, string(e.Type))

	return nil
}

func (s *Subscriber) grantBadges(ctx context.Context, userID uuid.UUID) {
	if s.badges == nil {
		return
	}

	if err := s.badges.AwardEarned(ctx, userID); err != nil {
		slog.ErrorContext(ctx, "award badges failed",
			slog.String("user_id", userID.String()),
			slog.Any("error", err),
		)
	}
}

func (s *Subscriber) notify(userID uuid.UUID, pet *domain.Pet, progress domain.Progress, reason string) {
	if s.notifier == nil || pet == nil {
		return
	}

	s.notifier.PetUpdated(userID, pet)

	if progress.XPGranted > 0 {
		s.notifier.XPGained(userID, progress.XPGranted, reason, pet.XP())
	}

	if progress.Level > progress.PreviousLevel {
		s.notifier.LevelUp(userID, progress.Level)
	}
}

func (s *Subscriber) grantRewards(ctx context.Context, userID uuid.UUID, progress domain.Progress) {
	if s.rewards == nil || progress.Level <= progress.PreviousLevel {
		return
	}

	if _, err := s.rewards.GrantEligible(ctx, userID); err != nil {
		slog.ErrorContext(ctx, "grant level rewards failed",
			slog.String("user_id", userID.String()),
			slog.Any("error", err),
		)
	}
}

func isSkippable(err error) bool {
	return errors.Is(err, domain.ErrDuplicateAction) ||
		errors.Is(err, domain.ErrLimitReached) ||
		errors.Is(err, domain.ErrConditionNotMet) ||
		errors.Is(err, domain.ErrInvalidAction) ||
		errors.Is(err, domainerr.ErrNotFound)
}
