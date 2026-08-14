package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

type HatchNotifier interface {
	PetHatched(userID uuid.UUID, pet *domain.Pet)
}

type Service struct {
	pets     domain.Repository
	journal  XPJournal
	cache    Cache
	hot      HotStateStore
	tx       TxManager
	clock    Clock
	notifier HatchNotifier
	accounts AccountGate
}

func NewService(pets domain.Repository, journal XPJournal, cache Cache, tx TxManager, clock Clock) *Service {
	return &Service{pets: pets, journal: journal, cache: cache, tx: tx, clock: clock}
}

func (s *Service) WithHatchNotifier(notifier HatchNotifier) *Service {
	s.notifier = notifier

	return s
}

func (s *Service) WithAccountGate(accounts AccountGate) *Service {
	s.accounts = accounts

	return s
}

func (s *Service) WithHotState(hot HotStateStore) *Service {
	s.hot = hot

	return s
}

func (s *Service) State(ctx context.Context, userID uuid.UUID) (*domain.Pet, error) {
	pet, err := s.readPet(ctx, userID)
	if err != nil {
		return nil, err
	}
	pet.ApplyDecay(s.clock.Now())
	s.mergeHotState(ctx, pet)

	return pet, nil
}

func (s *Service) readPet(ctx context.Context, userID uuid.UUID) (*domain.Pet, error) {
	cached, err := s.cache.Get(ctx, userID)
	if err == nil && cached != nil {
		return cached, nil
	}

	pet, err := s.pets.ByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	s.refreshCache(ctx, pet)

	return pet, nil
}

func (s *Service) mergeHotState(ctx context.Context, pet *domain.Pet) {
	if s.hot == nil || pet == nil {
		return
	}

	hot, err := s.hot.GetOrInitialize(ctx, hotFromPet(pet))
	if err != nil {
		slog.WarnContext(ctx, "load hot pet state",
			slog.String("user_id", pet.UserID().String()), slog.Any("error", err))

		return
	}
	if err := pet.ApplyHotState(hot.Happiness, hot.Satiety, hot.Version, hot.UpdatedAt); err != nil {
		slog.WarnContext(ctx, "apply hot pet state",
			slog.String("user_id", pet.UserID().String()), slog.Any("error", err))
	}
}

func hotFromPet(pet *domain.Pet) HotState {
	return HotState{
		UserID: pet.UserID(), Happiness: pet.Happiness(), Satiety: pet.Satiety(),
		Version: pet.InteractionVersion(), UpdatedAt: pet.UpdatedAt(),
	}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID) (*domain.Pet, error) {
	return s.mutate(ctx, userID, func(_ context.Context, pet *domain.Pet) error {
		pet.ApplyDecay(s.clock.Now())

		return nil
	})
}

func (s *Service) hatching(
	ctx context.Context,
	userID uuid.UUID,
	apply func(context.Context, *domain.Pet) error,
) (*domain.Pet, error) {
	hatched := false

	pet, err := s.mutate(ctx, userID, func(ctx context.Context, pet *domain.Pet) error {
		if applyErr := apply(ctx, pet); applyErr != nil {
			return applyErr
		}
		hatched = pet.Hatch(s.clock.Now())

		return nil
	})
	if err != nil {
		return nil, err
	}

	if hatched && s.notifier != nil {
		s.notifier.PetHatched(userID, pet)
	}

	return pet, nil
}

func (s *Service) mutate(
	ctx context.Context,
	userID uuid.UUID,
	apply func(context.Context, *domain.Pet) error,
) (*domain.Pet, error) {
	var result *domain.Pet

	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		pet, err := s.load(ctx, userID)
		if err != nil {
			return err
		}
		if err := apply(ctx, pet); err != nil {
			return err
		}
		if err := s.pets.Save(ctx, pet); err != nil {
			return err
		}
		result = pet

		return nil
	})
	if err != nil {
		s.invalidateCache(ctx, userID)

		return nil, err
	}

	s.refreshCache(ctx, result)

	return result, nil
}

func (s *Service) refreshCache(ctx context.Context, pet *domain.Pet) {
	if pet == nil {
		return
	}

	if err := s.cache.Set(ctx, pet); err != nil {
		slog.WarnContext(ctx, "cache pet after commit",
			slog.String("user_id", pet.UserID().String()), slog.Any("error", err))
	}
}

func (s *Service) invalidateCache(ctx context.Context, userID uuid.UUID) {
	if err := s.cache.Delete(ctx, userID); err != nil {
		slog.WarnContext(ctx, "invalidate pet cache",
			slog.String("user_id", userID.String()), slog.Any("error", err))
	}
}

func (s *Service) load(ctx context.Context, userID uuid.UUID) (*domain.Pet, error) {
	pet, err := s.pets.ByUserIDForUpdate(ctx, userID)
	if err != nil && !errors.Is(err, domainerr.ErrNotFound) {
		return nil, fmt.Errorf("load pet: %w", err)
	}
	if errors.Is(err, domainerr.ErrNotFound) {
		return domain.New(userID, s.clock.Now()), nil
	}

	return pet, nil
}
