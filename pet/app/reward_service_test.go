package app_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/domain"
)

func petRepoAtLevel(t *testing.T, userID uuid.UUID, level int) *fakeRepository {
	t.Helper()

	repo := newFakeRepository()
	repo.pets[userID] = domain.Restore(domain.RestoreParams{
		ID: uuid.New(), UserID: userID, Stage: domain.StageBaby, Level: level,
		Satiety: 70, Happiness: 70, Energy: 100,
		LastDecayTime: testNow(), UpdatedAt: testNow(),
	})

	return repo
}

func TestRewardCatalogMarksUnlockedAndClaimed(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1), levelReward("r5", 5))
	granted, err := domain.GrantReward(userID, "r1", testNow())
	require.NoError(t, err)
	repo.grants[grantKey(userID, "r1")] = granted

	service := newRewardService(repo, petRepoAtLevel(t, userID, 3), &stubSigner{}, nil)

	entries, err := service.Catalog(t.Context(), userID)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.True(t, entries[0].Unlocked)
	assert.True(t, entries[0].Claimed)
	assert.Equal(t, domain.RewardGranted, entries[0].Status)

	assert.False(t, entries[1].Unlocked)
	assert.False(t, entries[1].Claimed)
	assert.Equal(t, 3, entries[1].Current)
	assert.Equal(t, 5, entries[1].Target)
}

func TestRewardCatalogPropagatesErrors(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")

	tests := map[string]func(*fakeRewardRepository, *fakeRepository){
		"catalog fails": func(r *fakeRewardRepository, _ *fakeRepository) { r.catalogErr = sentinel },
		"list fails":    func(r *fakeRewardRepository, _ *fakeRepository) { r.listErr = sentinel },
		"pet missing": func(_ *fakeRewardRepository, p *fakeRepository) {
			p.pets = map[uuid.UUID]*domain.Pet{}
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			userID := uuid.New()
			rewards := newFakeRewardRepository(levelReward("r1", 1))
			pets := petRepoAtLevel(t, userID, 3)
			mutate(rewards, pets)

			_, err := newRewardService(rewards, pets, &stubSigner{}, nil).Catalog(t.Context(), userID)
			require.Error(t, err)
		})
	}
}

func TestRewardMineJoinsCatalogDetails(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := newFakeRewardRepository(levelReward("r1", 1))
	granted, err := domain.GrantReward(userID, "r1", testNow())
	require.NoError(t, err)
	repo.grants[grantKey(userID, "r1")] = granted

	mine, err := newRewardService(repo, newFakeRepository(), &stubSigner{}, nil).Mine(t.Context(), userID)
	require.NoError(t, err)
	require.Len(t, mine, 1)

	assert.Equal(t, "r1", mine[0].Reward.ID())
	assert.Equal(t, "reward r1", mine[0].Reward.Title())
	assert.Equal(t, domain.RewardGranted, mine[0].Grant.Status())
}

func TestRewardMineReturnsEmptyForUnknownUser(t *testing.T) {
	t.Parallel()

	repo := newFakeRewardRepository(levelReward("r1", 1))

	mine, err := newRewardService(repo, newFakeRepository(), &stubSigner{}, nil).Mine(t.Context(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, mine)
}
