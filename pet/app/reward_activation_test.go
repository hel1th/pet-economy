package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

func TestGrantEligibleIssuesOnlyUnlockedRewardsOnce(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1), levelReward("r3", 3), levelReward("r9", 9))
	notifier := &spyRewardNotifier{}
	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, notifier)

	issued, err := service.GrantEligible(t.Context(), userID)
	require.NoError(t, err)
	require.Len(t, issued, 2)
	assert.Equal(t, []string{"r1", "r3"}, []string{issued[0].ID(), issued[1].ID()})
	assert.Equal(t, []string{"r1", "r3"}, notifier.granted)

	again, err := service.GrantEligible(t.Context(), userID)
	require.NoError(t, err)
	assert.Empty(t, again)
	assert.Len(t, notifier.granted, 2)
}

func TestGrantEligibleToleratesNilNotifier(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))

	issued, err := newRewardService(repo, petRepoAtLevel(t, userID, 2), &stubSigner{}, nil).
		GrantEligible(t.Context(), userID)

	require.NoError(t, err)
	assert.Len(t, issued, 1)
}

func TestGrantEligiblePropagatesRepositoryErrors(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("grant failed")
	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	repo.grantErr = sentinel

	_, err := newRewardService(repo, petRepoAtLevel(t, userID, 2), &stubSigner{}, nil).
		GrantEligible(t.Context(), userID)

	assert.ErrorIs(t, err, sentinel)
}

func TestActivateReturnsSignedCode(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	granted, err := domain.GrantReward(userID, "r1", testNow())
	require.NoError(t, err)
	repo.grants[grantKey(userID, "r1")] = granted

	signer := &stubSigner{code: "PROMO-123"}
	code, err := newRewardService(repo, newFakeRepository(), signer, nil).Activate(t.Context(), userID, "r1")

	require.NoError(t, err)
	assert.Equal(t, "PROMO-123", code)
	assert.Equal(t, []string{"r1"}, signer.issued)
	assert.Equal(t, []string{"r1"}, signer.verified)
	assert.True(t, repo.grants[grantKey(userID, "r1")].IsActivated())
}

func TestActivateIsNotRepeatable(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	granted, err := domain.GrantReward(userID, "r1", testNow())
	require.NoError(t, err)
	repo.grants[grantKey(userID, "r1")] = granted

	service := newRewardService(repo, newFakeRepository(), &stubSigner{}, nil)

	_, err = service.Activate(t.Context(), userID, "r1")
	require.NoError(t, err)

	_, err = service.Activate(t.Context(), userID, "r1")
	assert.ErrorIs(t, err, domain.ErrRewardAlreadyActivated)
}

func TestActivateRejections(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("infra down")
	userID := uuid.New()

	tests := []struct {
		name     string
		rewardID string
		signer   *stubSigner
		nilSign  bool
		mutate   func(*fakeRewardRepository)
		wantErr  error
	}{
		{
			name:     "empty reward id",
			rewardID: "",
			signer:   &stubSigner{},
			wantErr:  &domainerr.InvalidError{},
		},
		{
			name:     "reward not granted",
			rewardID: "missing",
			signer:   &stubSigner{},
			wantErr:  domain.ErrRewardNotGranted,
		},
		{
			name:     "repository failure",
			rewardID: "r1",
			signer:   &stubSigner{},
			mutate:   func(r *fakeRewardRepository) { r.byUserErr = sentinel },
			wantErr:  sentinel,
		},
		{
			name:     "issue failure",
			rewardID: "r1",
			signer:   &stubSigner{issueErr: sentinel},
			wantErr:  sentinel,
		},
		{
			name:     "verify failure",
			rewardID: "r1",
			signer:   &stubSigner{verifyErr: domain.ErrForgedCode},
			wantErr:  domain.ErrForgedCode,
		},
		{
			name:     "activate rejected by storage",
			rewardID: "r1",
			signer:   &stubSigner{},
			mutate: func(r *fakeRewardRepository) {
				no := false
				r.activateOK = &no
			},
			wantErr: domain.ErrRewardAlreadyActivated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := newFakeRewardRepository(levelReward("r1", 1))
			granted, err := domain.GrantReward(userID, "r1", testNow())
			require.NoError(t, err)
			repo.grants[grantKey(userID, "r1")] = granted

			if tt.mutate != nil {
				tt.mutate(repo)
			}

			_, err = newRewardService(repo, newFakeRepository(), tt.signer, nil).
				Activate(t.Context(), userID, tt.rewardID)

			require.Error(t, err)

			var invalid *domainerr.InvalidError
			if errors.As(tt.wantErr, &invalid) {
				assert.ErrorAs(t, err, &invalid)

				return
			}

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestActivateWithoutSignerFails(t *testing.T) {
	t.Parallel()

	repo := newFakeRewardRepository(levelReward("r1", 1))

	_, err := newRewardService(repo, newFakeRepository(), nil, nil).
		Activate(context.Background(), uuid.New(), "r1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "signer")
}
