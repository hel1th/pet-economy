package app_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/domain"
)

func TestMinePropagatesListFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("grants table unavailable")
	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	repo.listErr = sentinel

	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, nil)

	mine, err := service.Mine(t.Context(), userID)

	require.ErrorIs(t, err, sentinel)
	assert.Nil(t, mine)
}

func TestMinePropagatesCatalogFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("catalog table unavailable")
	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	granted, err := domain.GrantReward(userID, "r1", testNow())
	require.NoError(t, err)
	repo.grants[grantKey(userID, "r1")] = granted
	repo.catalogErr = sentinel

	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, nil)

	mine, err := service.Mine(t.Context(), userID)

	require.ErrorIs(t, err, sentinel)
	assert.Nil(t, mine)
}

func TestMineReturnsEmptySliceWithoutGrants(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, nil)

	mine, err := service.Mine(t.Context(), userID)

	require.NoError(t, err)
	assert.Empty(t, mine)
}

func TestMineJoinsGrantWithCatalogEntry(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	granted, err := domain.GrantReward(userID, "r1", testNow())
	require.NoError(t, err)
	repo.grants[grantKey(userID, "r1")] = granted

	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, nil)

	mine, err := service.Mine(t.Context(), userID)

	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, "r1", mine[0].Grant.RewardID())
	assert.Equal(t, "reward r1", mine[0].Reward.Title())
}

func TestMineKeepsGrantsMissingFromCatalog(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository()
	granted, err := domain.GrantReward(userID, "ghost", testNow())
	require.NoError(t, err)
	repo.grants[grantKey(userID, "ghost")] = granted

	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, nil)

	mine, err := service.Mine(t.Context(), userID)

	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, "ghost", mine[0].Grant.RewardID())
	assert.Empty(t, mine[0].Reward.Title())
}

func TestGrantEligiblePropagatesGrantFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("grant insert rejected")
	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	repo.grantErr = sentinel

	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, nil)

	issued, err := service.GrantEligible(t.Context(), userID)

	require.ErrorIs(t, err, sentinel)
	assert.Nil(t, issued)
}

func TestGrantEligibleSkipsAlreadyGrantedRewards(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	granted, err := domain.GrantReward(userID, "r1", testNow())
	require.NoError(t, err)
	repo.grants[grantKey(userID, "r1")] = granted

	notifier := &spyRewardNotifier{}
	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, notifier)

	issued, err := service.GrantEligible(t.Context(), userID)

	require.NoError(t, err)
	assert.Empty(t, issued)
	assert.Empty(t, notifier.granted)
}

func TestGrantEligibleWithoutNotifierStillGrants(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, nil)

	issued, err := service.GrantEligible(t.Context(), userID)

	require.NoError(t, err)
	require.Len(t, issued, 1)
	assert.Equal(t, "r1", issued[0].ID())
}

func TestActivateRequiresRewardID(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, nil)

	code, err := service.Activate(t.Context(), userID, "")

	require.Error(t, err)
	assert.Empty(t, code)
}

func TestActivateRequiresSigner(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), nil, nil)

	code, err := service.Activate(t.Context(), userID, "r1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "signer is not configured")
	assert.Empty(t, code)
}

func TestActivatePropagatesLookupFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("grant lookup failed")
	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	repo.byUserErr = sentinel

	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, nil)

	_, err := service.Activate(t.Context(), userID, "r1")

	require.ErrorIs(t, err, sentinel)
}

func TestActivatePropagatesSignerFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("hmac key missing")
	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	granted, err := domain.GrantReward(userID, "r1", testNow())
	require.NoError(t, err)
	repo.grants[grantKey(userID, "r1")] = granted

	service := newRewardService(
		repo, petRepoAtLevel(t, userID, 3), &stubSigner{issueErr: sentinel}, nil)

	_, err = service.Activate(t.Context(), userID, "r1")

	require.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "issue reward code")
}

func TestActivatePropagatesVerificationFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("signature mismatch")
	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	granted, err := domain.GrantReward(userID, "r1", testNow())
	require.NoError(t, err)
	repo.grants[grantKey(userID, "r1")] = granted

	service := newRewardService(
		repo, petRepoAtLevel(t, userID, 3), &stubSigner{verifyErr: sentinel}, nil)

	_, err = service.Activate(t.Context(), userID, "r1")

	require.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "verify reward code")
}

func TestActivatePropagatesRepositoryUpdateFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("update conflict")
	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	granted, err := domain.GrantReward(userID, "r1", testNow())
	require.NoError(t, err)
	repo.grants[grantKey(userID, "r1")] = granted
	repo.activErr = sentinel

	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, nil)

	_, err = service.Activate(t.Context(), userID, "r1")

	require.ErrorIs(t, err, sentinel)
}
