package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewardEnumsValidity(t *testing.T) {
	t.Parallel()

	for _, kind := range []RewardKind{
		RewardKindPromo, RewardKindUtility, RewardKindCosmetic,
	} {
		assert.True(t, kind.Valid(), string(kind))
	}
	assert.False(t, RewardKind("").Valid())
	assert.False(t, RewardKind("mystery").Valid())

	for _, condition := range []ConditionType{
		ConditionLevel, ConditionStreak, ConditionAchievement,
	} {
		assert.True(t, condition.Valid(), string(condition))
	}
	assert.False(t, ConditionType("karma").Valid())

	for _, status := range []RewardStatus{
		RewardGranted, RewardActivated, RewardExpired,
	} {
		assert.True(t, status.Valid(), string(status))
	}
	assert.False(t, RewardStatus("pending").Valid())
}

func TestNewRewardValidation(t *testing.T) {
	t.Parallel()

	valid := RewardParams{
		ID: "free_delivery", Title: "Free delivery", Description: "Ship for free",
		Kind: RewardKindPromo, ConditionType: ConditionLevel, ConditionValue: 10,
	}

	tests := []struct {
		name    string
		mutate  func(*RewardParams)
		wantErr bool
	}{
		{name: "valid reward"},
		{name: "zero condition value is allowed", mutate: func(p *RewardParams) { p.ConditionValue = 0 }},
		{name: "empty id", mutate: func(p *RewardParams) { p.ID = "" }, wantErr: true},
		{name: "unknown kind", mutate: func(p *RewardParams) { p.Kind = "misc" }, wantErr: true},
		{name: "unknown condition", mutate: func(p *RewardParams) { p.ConditionType = "karma" }, wantErr: true},
		{name: "negative condition", mutate: func(p *RewardParams) { p.ConditionValue = -1 }, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params := valid
			if tc.mutate != nil {
				tc.mutate(&params)
			}

			reward, err := NewReward(params)

			if tc.wantErr {
				require.ErrorIs(t, err, ErrInvalidReward)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, params.ID, reward.ID())
			assert.Equal(t, params.Title, reward.Title())
			assert.Equal(t, params.Description, reward.Description())
			assert.Equal(t, params.Kind, reward.Kind())
			assert.Equal(t, params.ConditionType, reward.ConditionType())
			assert.Equal(t, params.ConditionValue, reward.ConditionValue())
		})
	}
}

func TestGrantReward(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	now := time.Date(2026, time.May, 1, 8, 0, 0, 0, time.UTC)

	granted, err := GrantReward(userID, "free_delivery", now)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, granted.ID())
	assert.Equal(t, userID, granted.UserID())
	assert.Equal(t, "free_delivery", granted.RewardID())
	assert.Equal(t, RewardGranted, granted.Status())
	assert.Equal(t, now, granted.GrantedAt())
	assert.Empty(t, granted.Code())
	assert.Nil(t, granted.ActivatedAt())
	assert.Nil(t, granted.ExpiresAt())
	assert.False(t, granted.IsActivated())

	_, err = GrantReward(uuid.Nil, "free_delivery", now)
	require.ErrorIs(t, err, ErrInvalidReward)

	_, err = GrantReward(userID, "", now)
	require.ErrorIs(t, err, ErrInvalidReward)
}

func TestUserRewardActivationLifecycle(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	now := time.Date(2026, time.May, 1, 8, 0, 0, 0, time.UTC)

	granted, err := GrantReward(userID, "free_delivery", now)
	require.NoError(t, err)

	require.NoError(t, granted.Activate("PROMO-123", now.Add(time.Minute)))
	assert.True(t, granted.IsActivated())
	assert.Equal(t, "PROMO-123", granted.Code())
	require.NotNil(t, granted.ActivatedAt())
	assert.Equal(t, now.Add(time.Minute), *granted.ActivatedAt())

	require.ErrorIs(t, granted.Activate("PROMO-456", now.Add(time.Hour)), ErrRewardAlreadyActivated)
	assert.Equal(t, "PROMO-123", granted.Code())
}

func TestUserRewardActivationRejections(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	now := time.Date(2026, time.May, 1, 8, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Hour)

	tests := []struct {
		name      string
		params    UserRewardParams
		activated time.Time
		wantErr   error
	}{
		{
			name: "expired status is not activatable",
			params: UserRewardParams{
				ID: uuid.New(), UserID: userID, RewardID: "x", Status: RewardExpired, GrantedAt: now,
			},
			activated: now,
			wantErr:   ErrRewardNotActivatable,
		},
		{
			name: "unknown status is not activatable",
			params: UserRewardParams{
				ID: uuid.New(), UserID: userID, RewardID: "x", Status: "revoked", GrantedAt: now,
			},
			activated: now,
			wantErr:   ErrRewardNotActivatable,
		},
		{
			name: "expiration in the past",
			params: UserRewardParams{
				ID: uuid.New(), UserID: userID, RewardID: "x",
				Status: RewardGranted, GrantedAt: now, ExpiresAt: &expired,
			},
			activated: now,
			wantErr:   ErrRewardExpired,
		},
		{
			name: "expiration exactly now",
			params: UserRewardParams{
				ID: uuid.New(), UserID: userID, RewardID: "x",
				Status: RewardGranted, GrantedAt: now, ExpiresAt: &now,
			},
			activated: now,
			wantErr:   ErrRewardExpired,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reward := RestoreUserReward(tc.params)

			require.ErrorIs(t, reward.Activate("CODE", tc.activated), tc.wantErr)
			assert.False(t, reward.IsActivated())
		})
	}
}

func TestUserRewardActivatesBeforeExpiration(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 1, 8, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)

	reward := RestoreUserReward(UserRewardParams{
		ID: uuid.New(), UserID: uuid.New(), RewardID: "x",
		Status: RewardGranted, GrantedAt: now, ExpiresAt: &future,
	})

	require.NoError(t, reward.Activate("CODE", now))
	assert.True(t, reward.IsActivated())
	require.NotNil(t, reward.ExpiresAt())
	assert.Equal(t, future, *reward.ExpiresAt())
}

func TestRestoreUserRewardClonesTimestamps(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 1, 8, 0, 0, 0, time.UTC)
	activated := now.Add(time.Hour)

	reward := RestoreUserReward(UserRewardParams{
		ID: uuid.New(), UserID: uuid.New(), RewardID: "x", Status: RewardActivated,
		Code: "CODE", GrantedAt: now, ActivatedAt: &activated,
	})

	returned := reward.ActivatedAt()
	require.NotNil(t, returned)
	*returned = now.Add(100 * time.Hour)

	assert.Equal(t, activated, *reward.ActivatedAt())
}
