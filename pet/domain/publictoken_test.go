package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/domain"
)

func TestPublicTokenIsStable(t *testing.T) {
	id := uuid.New()

	first := domain.PublicToken(id)
	second := domain.PublicToken(id)

	require.NotEmpty(t, first)
	assert.Equal(t, first, second)
}

func TestPublicTokenHidesUUID(t *testing.T) {
	id := uuid.New()

	token := domain.PublicToken(id)

	assert.NotEqual(t, id.String(), token)
	assert.NotContains(t, token, id.String())
	assert.Len(t, token, 16)
}

func TestPublicTokenIsUniquePerID(t *testing.T) {
	seen := make(map[string]struct{})

	for range 100 {
		token := domain.PublicToken(uuid.New())
		_, duplicate := seen[token]
		require.False(t, duplicate)
		seen[token] = struct{}{}
	}
}

func TestPublicTokenNilIsEmpty(t *testing.T) {
	assert.Empty(t, domain.PublicToken(uuid.Nil))
}

func TestPublicTokenChangesWithSecret(t *testing.T) {
	id := uuid.New()

	domain.SetPublicTokenSecret("secret-one-000000000000")
	first := domain.PublicToken(id)

	domain.SetPublicTokenSecret("secret-two-000000000000")
	second := domain.PublicToken(id)

	defer domain.SetPublicTokenSecret("")

	assert.NotEqual(t, first, second)
}
