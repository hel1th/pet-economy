package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

type RewardCatalogEntry struct {
	Reward   domain.Reward
	Unlocked bool
	Claimed  bool
	Status   domain.RewardStatus
	Current  int
	Target   int
}

type GrantedReward struct {
	Reward domain.Reward
	Grant  domain.UserReward
}

type RewardService struct {
	rewards  RewardRepository
	pets     domain.Repository
	signer   CodeSigner
	tx       TxManager
	clock    Clock
	notifier RewardNotifier
}

type RewardServiceDeps struct {
	Rewards  RewardRepository
	Pets     domain.Repository
	Signer   CodeSigner
	Tx       TxManager
	Clock    Clock
	Notifier RewardNotifier
}

func NewRewardService(deps RewardServiceDeps) *RewardService {
	return &RewardService{
		rewards: deps.Rewards, pets: deps.Pets, signer: deps.Signer,
		tx: deps.Tx, clock: deps.Clock, notifier: deps.Notifier,
	}
}

func (s *RewardService) Catalog(ctx context.Context, userID uuid.UUID) ([]RewardCatalogEntry, error) {
	catalog, err := s.rewards.Catalog(ctx)
	if err != nil {
		return nil, err
	}

	pet, err := s.petOf(ctx, userID)
	if err != nil {
		return nil, err
	}

	claimed, err := s.claimedStatuses(ctx, userID)
	if err != nil {
		return nil, err
	}

	entries := make([]RewardCatalogEntry, 0, len(catalog))
	for _, reward := range catalog {
		progress := domain.EvaluateRewardProgress(reward, pet)
		status, ok := claimed[reward.ID()]
		entries = append(entries, RewardCatalogEntry{
			Reward: reward, Unlocked: progress.Unlocked, Claimed: ok,
			Status: status, Current: progress.Current, Target: progress.Target,
		})
	}

	return entries, nil
}

func (s *RewardService) Mine(ctx context.Context, userID uuid.UUID) ([]GrantedReward, error) {
	granted, err := s.rewards.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	catalog, err := s.rewards.Catalog(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]domain.Reward, len(catalog))
	for _, reward := range catalog {
		byID[reward.ID()] = reward
	}

	mine := make([]GrantedReward, 0, len(granted))
	for _, item := range granted {
		mine = append(mine, GrantedReward{Reward: byID[item.RewardID()], Grant: item})
	}

	return mine, nil
}

func (s *RewardService) GrantEligible(ctx context.Context, userID uuid.UUID) ([]domain.Reward, error) {
	pet, err := s.petOf(ctx, userID)
	if err != nil {
		return nil, err
	}

	catalog, err := s.rewards.Catalog(ctx)
	if err != nil {
		return nil, err
	}

	issued := make([]domain.Reward, 0)
	for _, reward := range domain.EligibleRewards(catalog, pet) {
		granted, grantErr := domain.GrantReward(userID, reward.ID(), s.clock.Now())
		if grantErr != nil {
			return nil, grantErr
		}

		inserted, grantErr := s.rewards.Grant(ctx, granted)
		if grantErr != nil {
			return nil, grantErr
		}
		if inserted {
			issued = append(issued, reward)
			s.notifyGranted(userID, reward)
		}
	}

	return issued, nil
}

func (s *RewardService) Activate(ctx context.Context, userID uuid.UUID, rewardID string) (string, error) {
	if rewardID == "" {
		return "", domainerr.NewInvalid("reward_id", "reward id is required")
	}
	if s.signer == nil {
		return "", errors.New("reward signer is not configured")
	}

	var code string
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		issued, activateErr := s.activate(ctx, userID, rewardID)
		code = issued

		return activateErr
	})
	if err != nil {
		return "", err
	}

	return code, nil
}

func (s *RewardService) activate(ctx context.Context, userID uuid.UUID, rewardID string) (string, error) {
	granted, err := s.rewards.ByUserAndReward(ctx, userID, rewardID)
	if err != nil {
		if errors.Is(err, domainerr.ErrNotFound) {
			return "", domain.ErrRewardNotGranted
		}

		return "", err
	}

	code, err := s.signer.Issue(userID, rewardID)
	if err != nil {
		return "", fmt.Errorf("issue reward code: %w", err)
	}

	if activateErr := granted.Activate(code, s.clock.Now()); activateErr != nil {
		return "", activateErr
	}

	updated, err := s.rewards.Activate(ctx, granted)
	if err != nil {
		return "", err
	}
	if !updated {
		return "", domain.ErrRewardAlreadyActivated
	}

	if err := s.signer.Verify(userID, rewardID, code); err != nil {
		return "", fmt.Errorf("verify reward code: %w", err)
	}

	return code, nil
}

func (s *RewardService) petOf(ctx context.Context, userID uuid.UUID) (*domain.Pet, error) {
	pet, err := s.pets.ByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	pet.ApplyDecay(s.clock.Now())

	return pet, nil
}

func (s *RewardService) claimedStatuses(
	ctx context.Context,
	userID uuid.UUID,
) (map[string]domain.RewardStatus, error) {
	granted, err := s.rewards.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	statuses := make(map[string]domain.RewardStatus, len(granted))
	for _, item := range granted {
		statuses[item.RewardID()] = item.Status()
	}

	return statuses, nil
}

func (s *RewardService) notifyGranted(userID uuid.UUID, reward domain.Reward) {
	if s.notifier == nil {
		return
	}

	s.notifier.RewardGranted(userID, reward.ID(), reward.Title())
}
