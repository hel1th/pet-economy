package app_test

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

type fakeRewardRepository struct {
	mu         sync.Mutex
	catalog    []domain.Reward
	grants     map[string]domain.UserReward
	catalogErr error
	grantErr   error
	listErr    error
	byUserErr  error
	activErr   error
	activateOK *bool
	grantCalls int
}

func newFakeRewardRepository(catalog ...domain.Reward) *fakeRewardRepository {
	return &fakeRewardRepository{
		catalog: catalog,
		grants:  map[string]domain.UserReward{},
	}
}

func grantKey(userID uuid.UUID, rewardID string) string {
	return userID.String() + "|" + rewardID
}

func (r *fakeRewardRepository) Catalog(context.Context) ([]domain.Reward, error) {
	if r.catalogErr != nil {
		return nil, r.catalogErr
	}

	return r.catalog, nil
}

func (r *fakeRewardRepository) ByID(_ context.Context, rewardID string) (domain.Reward, error) {
	for _, reward := range r.catalog {
		if reward.ID() == rewardID {
			return reward, nil
		}
	}

	return domain.Reward{}, domainerr.ErrNotFound
}

func (r *fakeRewardRepository) Grant(_ context.Context, granted domain.UserReward) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.grantCalls++

	if r.grantErr != nil {
		return false, r.grantErr
	}

	k := grantKey(granted.UserID(), granted.RewardID())
	if _, ok := r.grants[k]; ok {
		return false, nil
	}
	r.grants[k] = granted

	return true, nil
}

func (r *fakeRewardRepository) ListByUser(_ context.Context, userID uuid.UUID) ([]domain.UserReward, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.listErr != nil {
		return nil, r.listErr
	}

	out := make([]domain.UserReward, 0, len(r.grants))
	for _, grant := range r.grants {
		if grant.UserID() == userID {
			out = append(out, grant)
		}
	}

	return out, nil
}

func (r *fakeRewardRepository) ByUserAndReward(
	_ context.Context,
	userID uuid.UUID,
	rewardID string,
) (domain.UserReward, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.byUserErr != nil {
		return domain.UserReward{}, r.byUserErr
	}

	grant, ok := r.grants[grantKey(userID, rewardID)]
	if !ok {
		return domain.UserReward{}, domainerr.ErrNotFound
	}

	return grant, nil
}

func (r *fakeRewardRepository) Activate(_ context.Context, granted domain.UserReward) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.activErr != nil {
		return false, r.activErr
	}
	if r.activateOK != nil {
		return *r.activateOK, nil
	}

	k := grantKey(granted.UserID(), granted.RewardID())
	stored, ok := r.grants[k]
	if !ok || stored.IsActivated() {
		return false, nil
	}
	r.grants[k] = granted

	return true, nil
}

type stubSigner struct {
	code      string
	issueErr  error
	verifyErr error
	issued    []string
	verified  []string
}

func (s *stubSigner) Issue(_ uuid.UUID, rewardID string) (string, error) {
	if s.issueErr != nil {
		return "", s.issueErr
	}
	s.issued = append(s.issued, rewardID)

	code := s.code
	if code == "" {
		code = "code-" + rewardID
	}

	return code, nil
}

func (s *stubSigner) Verify(_ uuid.UUID, rewardID, _ string) error {
	s.verified = append(s.verified, rewardID)

	return s.verifyErr
}

type spyRewardNotifier struct {
	mu      sync.Mutex
	granted []string
}

func (n *spyRewardNotifier) RewardGranted(_ uuid.UUID, rewardID, _ string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.granted = append(n.granted, rewardID)
}

func newRewardService(
	repo app.RewardRepository,
	pets domain.Repository,
	signer app.CodeSigner,
	notifier app.RewardNotifier,
) *app.RewardService {
	return app.NewRewardService(app.RewardServiceDeps{
		Rewards:  repo,
		Pets:     pets,
		Signer:   signer,
		Tx:       passthroughTx{},
		Clock:    fixedClock{now: testNow()},
		Notifier: notifier,
	})
}

func levelReward(id string, level int) domain.Reward {
	reward, err := domain.NewReward(domain.RewardParams{
		ID:             id,
		Title:          "reward " + id,
		Description:    "unlocked at level",
		Kind:           domain.RewardKindPromo,
		ConditionType:  domain.ConditionLevel,
		ConditionValue: level,
	})
	if err != nil {
		panic(err)
	}

	return reward
}
