package domain_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/games/domain"
	"github.com/hel1th/pet-economy/shared/publicid"
)

type stubGame struct {
	slug   string
	target int
}

func (g stubGame) Slug() string      { return g.slug }
func (g stubGame) TargetStreak() int { return g.target }

func (g stubGame) Start(context.Context, *domain.Round) (domain.View, error) {
	return domain.View{Prompt: json.RawMessage(`{}`)}, nil
}

func (g stubGame) Resume(context.Context, *domain.Round) (domain.View, error) {
	return domain.View{Prompt: json.RawMessage(`{}`)}, nil
}

func (g stubGame) Guess(context.Context, *domain.Round, json.RawMessage) (domain.GuessOutcome, error) {
	return domain.GuessOutcome{}, nil
}

func TestNewRoundStartsActiveWithValidDisplayID(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 14, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()

	round := domain.NewRound(userID, "moreless", now)

	assert.NotEqual(t, uuid.Nil, round.ID())
	assert.True(t, publicid.IsValid(round.DisplayID()))
	assert.Equal(t, userID, round.UserID())
	assert.Equal(t, "moreless", round.GameSlug())
	assert.Equal(t, domain.StateActive, round.State())
	assert.True(t, round.IsActive())
	assert.Equal(t, 0, round.Streak())
	assert.JSONEq(t, `{}`, string(round.Payload()))
}

func TestRoundAdvanceReachesTargetAndWins(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 14, 12, 0, 0, 0, time.UTC)
	round := domain.NewRound(uuid.New(), "moreless", now)

	for i := 1; i < 7; i++ {
		round.Advance(7, now)
		assert.Equal(t, i, round.Streak())
		assert.Equal(t, domain.StateActive, round.State())
	}

	round.Advance(7, now)

	assert.Equal(t, 7, round.Streak())
	assert.Equal(t, 7, round.BestStreak())
	assert.Equal(t, domain.StateWon, round.State())
	assert.False(t, round.IsActive())
}

func TestRoundWinWithoutStreak(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 14, 12, 0, 0, 0, time.UTC)
	round := domain.NewRound(uuid.New(), "bukovki", now)

	round.Win(now.Add(time.Minute))

	assert.Equal(t, domain.StateWon, round.State())
	assert.False(t, round.IsActive())
	assert.Equal(t, 0, round.Streak(), "winning must not fabricate a streak")
	assert.Equal(t, 0, round.BestStreak())
	assert.Equal(t, now.Add(time.Minute), round.UpdatedAt())
}

func TestRoundAdvanceWithoutTargetNeverWins(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 14, 12, 0, 0, 0, time.UTC)
	round := domain.NewRound(uuid.New(), "bukovki", now)

	round.Advance(0, now)

	assert.Equal(t, 1, round.Streak())
	assert.Equal(t, domain.StateActive, round.State())
}

func TestRoundLose(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 14, 12, 0, 0, 0, time.UTC)
	round := domain.NewRound(uuid.New(), "moreless", now)

	round.Advance(7, now)
	round.Lose(now.Add(time.Minute))

	assert.Equal(t, domain.StateLost, round.State())
	assert.Equal(t, 1, round.BestStreak())
	assert.False(t, round.IsActive())
}

func TestRoundSetPayloadDefaultsToEmptyObject(t *testing.T) {
	t.Parallel()

	round := domain.NewRound(uuid.New(), "moreless", time.Now())

	round.SetPayload(nil)
	assert.JSONEq(t, `{}`, string(round.Payload()))

	round.SetPayload(json.RawMessage(`{"seen":["a"]}`))
	assert.JSONEq(t, `{"seen":["a"]}`, string(round.Payload()))
}

func TestStateValid(t *testing.T) {
	t.Parallel()

	assert.True(t, domain.StateActive.Valid())
	assert.True(t, domain.StateWon.Valid())
	assert.True(t, domain.StateLost.Valid())
	assert.False(t, domain.State("paused").Valid())
}

func TestRegistryLookupAndOrder(t *testing.T) {
	t.Parallel()

	registry := domain.NewRegistry(stubGame{slug: "moreless", target: 7}, stubGame{slug: "guessyear", target: 5})

	game, err := registry.Get("moreless")
	require.NoError(t, err)
	assert.Equal(t, 7, game.TargetStreak())

	assert.Equal(t, []string{"moreless", "guessyear"}, registry.Slugs())
	assert.Len(t, registry.All(), 2)
}

func TestRegistryUnknownSlugIsNotFound(t *testing.T) {
	t.Parallel()

	registry := domain.NewRegistry(stubGame{slug: "moreless", target: 7})

	_, err := registry.Get("nope")

	require.ErrorIs(t, err, domain.ErrGameNotFound)
}

func TestRegistryReplacingSlugKeepsOrderStable(t *testing.T) {
	t.Parallel()

	registry := domain.NewRegistry(stubGame{slug: "a", target: 1}, stubGame{slug: "b", target: 2})
	registry.Register(stubGame{slug: "a", target: 9})

	assert.Equal(t, []string{"a", "b"}, registry.Slugs())

	game, err := registry.Get("a")
	require.NoError(t, err)
	assert.Equal(t, 9, game.TargetStreak())
}
