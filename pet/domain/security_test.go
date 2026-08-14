package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "reward-secret-for-tests"

func TestRewardSignerRejectsWeakSecret(t *testing.T) {
	t.Parallel()

	signer, err := NewRewardSigner("short")
	require.ErrorIs(t, err, ErrWeakSecret)
	assert.Nil(t, signer)
}

func TestRewardCodeRoundTrip(t *testing.T) {
	t.Parallel()

	signer, err := NewRewardSigner(testSecret)
	require.NoError(t, err)

	userID := uuid.New()
	code, err := signer.Issue(userID, "free_delivery_500")
	require.NoError(t, err)
	require.NotEmpty(t, code)

	assert.NoError(t, signer.Verify(userID, "free_delivery_500", code))
}

func TestRewardCodeRejections(t *testing.T) {
	t.Parallel()

	signer, err := NewRewardSigner(testSecret)
	require.NoError(t, err)

	owner := uuid.New()
	stranger := uuid.New()
	code, err := signer.Issue(owner, "free_delivery_500")
	require.NoError(t, err)

	tests := []struct {
		name     string
		userID   uuid.UUID
		rewardID string
		code     string
		err      error
	}{
		{name: "foreign user", userID: stranger, rewardID: "free_delivery_500", code: code, err: ErrForgedCode},
		{name: "other reward", userID: owner, rewardID: "views_boost_24h", code: code, err: ErrForgedCode},
		{
			name: "tampered signature", userID: owner, rewardID: "free_delivery_500",
			code: tamperLastRune(code), err: ErrForgedCode,
		},
		{name: "no separator", userID: owner, rewardID: "free_delivery_500", code: "PLAINCODE", err: ErrInvalidCode},
		{name: "empty nonce", userID: owner, rewardID: "free_delivery_500", code: "-SIGNATURE", err: ErrInvalidCode},
		{name: "non base32 nonce", userID: owner, rewardID: "free_delivery_500", code: "10!!-SIGNATURE", err: ErrInvalidCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, signer.Verify(tt.userID, tt.rewardID, tt.code), tt.err)
		})
	}
}

func TestRewardCodesAreUnique(t *testing.T) {
	t.Parallel()

	signer, err := NewRewardSigner(testSecret)
	require.NoError(t, err)

	userID := uuid.New()
	first, err := signer.Issue(userID, "free_delivery_500")
	require.NoError(t, err)
	second, err := signer.Issue(userID, "free_delivery_500")
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

func TestRewardActivationOnlyFromGranted(t *testing.T) {
	t.Parallel()

	signer, err := NewRewardSigner(testSecret)
	require.NoError(t, err)

	userID := uuid.New()
	granted, err := GrantReward(userID, "free_delivery_500", testTime())
	require.NoError(t, err)

	code, err := signer.Issue(userID, granted.RewardID())
	require.NoError(t, err)
	require.NoError(t, signer.Verify(granted.UserID(), granted.RewardID(), code))
	require.NoError(t, granted.Activate(code, testTime()))

	assert.True(t, granted.IsActivated())
	require.ErrorIs(t, granted.Activate(code, testTime()), ErrRewardAlreadyActivated)
}

func tamperLastRune(code string) string {
	last := code[len(code)-1]
	if last == 'A' {
		return code[:len(code)-1] + "B"
	}

	return code[:len(code)-1] + "A"
}
