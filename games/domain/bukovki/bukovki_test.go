package bukovki

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/games/domain"
)

type mockWordPool struct {
	word  string
	valid bool
}

func (m *mockWordPool) RandomWord(ctx context.Context, minLength, maxLength int) (string, error) {
	return m.word, nil
}

func (m *mockWordPool) IsValidWord(ctx context.Context, word string, length int) bool {
	return m.valid
}

type mockListingPool struct {
	listings []Listing
	err      error
	calls    int
	words    []string
}

func (m *mockListingPool) ListingsByWord(ctx context.Context, word string, limit int) ([]Listing, error) {
	m.calls++
	m.words = append(m.words, word)

	if m.err != nil {
		return nil, m.err
	}

	return m.listings, nil
}

func newGame(word string, listings *mockListingPool) (*Game, *domain.Round) {
	var pool ListingPool
	if listings != nil {
		pool = listings
	}

	game := New(&mockWordPool{word: word, valid: true}, pool)
	round := domain.NewRound(uuid.New(), Slug, time.Now())

	return game, round
}

func statuses(feedback []LetterFeedback) []LetterStatus {
	out := make([]LetterStatus, 0, len(feedback))
	for _, f := range feedback {
		out = append(out, f.Status)
	}

	return out
}

func TestEvaluateGuessCyrillic(t *testing.T) {
	tests := []struct {
		name     string
		guess    string
		secret   string
		expected []LetterStatus
	}{
		{
			name:     "all correct",
			guess:    "диван",
			secret:   "диван",
			expected: []LetterStatus{StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect},
		},
		{
			name:     "all absent",
			guess:    "шорты",
			secret:   "диван",
			expected: []LetterStatus{StatusAbsent, StatusAbsent, StatusAbsent, StatusAbsent, StatusAbsent},
		},
		{
			name:     "shared letters in wrong positions",
			guess:    "книга",
			secret:   "диван",
			expected: []LetterStatus{StatusAbsent, StatusPresent, StatusPresent, StatusAbsent, StatusPresent},
		},
		{
			name:     "repeated letter counted once",
			guess:    "колос",
			secret:   "полка",
			expected: []LetterStatus{StatusPresent, StatusCorrect, StatusCorrect, StatusAbsent, StatusAbsent},
		},
		{
			name:     "yo folds into ye on both sides",
			guess:    NormalizeWord("ковер"),
			secret:   NormalizeWord("ковёр"),
			expected: []LetterStatus{StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect},
		},
		{
			name:     "yo guess against ye secret",
			guess:    NormalizeWord("ковёр"),
			secret:   NormalizeWord("ковер"),
			expected: []LetterStatus{StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect},
		},
		{
			name:     "four letter word",
			guess:    "стол",
			secret:   "стол",
			expected: []LetterStatus{StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect},
		},
		{
			name:     "eight letter word",
			guess:    "шкатулка",
			secret:   "шкатулка",
			expected: []LetterStatus{StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect, StatusCorrect},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateGuess(tt.guess, tt.secret)
			require.Len(t, result, len([]rune(tt.guess)))

			guessRunes := []rune(tt.guess)
			for i, r := range result {
				assert.Equal(t, string(guessRunes[i]), r.Char)
			}

			assert.Equal(t, tt.expected, statuses(result))
		})
	}
}

func TestPromptExposesRuneLengthNotBytes(t *testing.T) {
	for _, word := range []string{"стол", "диван", "гитара", "шкатулка"} {
		game, round := newGame(word, nil)

		view, err := game.Start(context.Background(), round)
		require.NoError(t, err)

		var prompt promptPayload
		require.NoError(t, json.Unmarshal(view.Prompt, &prompt))

		assert.Equal(t, len([]rune(word)), prompt.WordLength,
			"word_length must be the rune count for %q", word)
		assert.Equal(t, maxTries, prompt.MaxTries)
	}
}

func TestPromptNeverLeaksSecret(t *testing.T) {
	game, round := newGame("диван", nil)

	view, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	assert.NotContains(t, string(view.Prompt), "диван",
		"the secret must never be serialized into the prompt")
	assert.Contains(t, string(round.Payload()), "диван",
		"the secret stays server-side in the round payload")

	outcome, err := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"книга"}`))
	require.NoError(t, err)

	assert.NotContains(t, string(outcome.Reveal), "диван",
		"a mid-round reveal must never contain the secret")
	require.NotNil(t, outcome.Next)
	assert.NotContains(t, string(outcome.Next.Prompt), "диван",
		"the next prompt must never contain the secret")
}

func TestMidRoundRevealHasNoListings(t *testing.T) {
	listings := &mockListingPool{listings: []Listing{
		{DisplayID: "abc123", Title: "Угловой диван", PriceKopeks: 1500000, PhotoURL: "/uploads/abc123/1.jpg"},
	}}

	game, round := newGame("диван", listings)

	_, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	outcome, err := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"книга"}`))
	require.NoError(t, err)

	var reveal revealPayload
	require.NoError(t, json.Unmarshal(outcome.Reveal, &reveal))

	assert.False(t, reveal.GameOver)
	assert.Empty(t, reveal.Listings, "listings must not leak mid-round")
	assert.NotContains(t, string(outcome.Reveal), "abc123")
	assert.Equal(t, 0, listings.calls, "listings must not even be queried mid-round")
}

func TestWinRevealsSecretAndListings(t *testing.T) {
	listings := &mockListingPool{listings: []Listing{
		{DisplayID: "abc123", Title: "Угловой диван", PriceKopeks: 1500000, PhotoURL: "/uploads/abc123/1.jpg"},
		{DisplayID: "def456", Title: "Диван-кровать", PriceKopeks: 900000, PhotoURL: "/uploads/def456/1.jpg"},
	}}

	game, round := newGame("диван", listings)

	_, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	outcome, err := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"диван"}`))
	require.NoError(t, err)

	assert.Equal(t, domain.ProgressWin, outcome.Progress)

	var reveal revealPayload
	require.NoError(t, json.Unmarshal(outcome.Reveal, &reveal))

	assert.True(t, reveal.GameOver)
	assert.True(t, reveal.Win)
	require.NotNil(t, reveal.Secret)
	assert.Equal(t, "диван", *reveal.Secret)

	require.Len(t, reveal.Listings, 2)
	assert.Equal(t, "abc123", reveal.Listings[0].DisplayID)
	assert.Equal(t, "/uploads/abc123/1.jpg", reveal.Listings[0].PhotoURL)
	assert.Equal(t, int64(1500000), reveal.Listings[0].PriceKopeks)

	assert.NotContains(t, string(outcome.Reveal), "item_id",
		"internal uuids must never be exposed")
}

func TestLossRevealsSecretAndListings(t *testing.T) {
	listings := &mockListingPool{listings: []Listing{
		{DisplayID: "abc123", Title: "Угловой диван", PriceKopeks: 1500000, PhotoURL: "/uploads/abc123/1.jpg"},
	}}

	game, round := newGame("диван", listings)

	_, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	wrong := []string{"книга", "лампа", "комод", "чехол", "робот", "полка"}
	require.Len(t, wrong, maxTries)

	var outcome domain.GuessOutcome

	for _, word := range wrong {
		outcome, err = game.Guess(context.Background(), round, json.RawMessage(`{"guess":"`+word+`"}`))
		require.NoError(t, err)
	}

	assert.Equal(t, domain.ProgressLose, outcome.Progress)

	var reveal revealPayload
	require.NoError(t, json.Unmarshal(outcome.Reveal, &reveal))

	assert.True(t, reveal.GameOver)
	assert.False(t, reveal.Win)
	require.NotNil(t, reveal.Secret)
	assert.Equal(t, "диван", *reveal.Secret)
	assert.Len(t, reveal.Listings, 1)
}

func loseRound(t *testing.T, game *Game, round *domain.Round) domain.GuessOutcome {
	t.Helper()

	wrong := []string{"книга", "лампа", "комод", "чехол", "робот", "полка"}
	require.Len(t, wrong, maxTries)

	var outcome domain.GuessOutcome

	for _, word := range wrong {
		var err error

		outcome, err = game.Guess(context.Background(), round, json.RawMessage(`{"guess":"`+word+`"}`))
		require.NoError(t, err)
	}

	return outcome
}

func TestListingsAreRevealedOnBothTerminalPaths(t *testing.T) {
	payoff := []Listing{
		{DisplayID: "abc123", Title: "Угловой диван", PriceKopeks: 1500000, PhotoURL: "/uploads/abc123/1.jpg"},
		{DisplayID: "def456", Title: "Диван-кровать", PriceKopeks: 900000, PhotoURL: "/uploads/def456/1.jpg"},
	}

	tests := []struct {
		name             string
		play             func(t *testing.T, game *Game, round *domain.Round) domain.GuessOutcome
		expectedProgress domain.Progress
		expectedWin      bool
	}{
		{
			name: "win",
			play: func(t *testing.T, game *Game, round *domain.Round) domain.GuessOutcome {
				t.Helper()

				outcome, err := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"диван"}`))
				require.NoError(t, err)

				return outcome
			},
			expectedProgress: domain.ProgressWin,
			expectedWin:      true,
		},
		{
			name:             "attempts exhausted",
			play:             loseRound,
			expectedProgress: domain.ProgressLose,
			expectedWin:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listings := &mockListingPool{listings: payoff}
			game, round := newGame("диван", listings)

			_, err := game.Start(context.Background(), round)
			require.NoError(t, err)

			outcome := tt.play(t, game, round)

			assert.Equal(t, tt.expectedProgress, outcome.Progress)

			var reveal revealPayload
			require.NoError(t, json.Unmarshal(outcome.Reveal, &reveal))

			assert.True(t, reveal.GameOver)
			assert.Equal(t, tt.expectedWin, reveal.Win)
			require.NotNil(t, reveal.Secret)
			assert.Equal(t, "диван", *reveal.Secret)

			require.Len(t, reveal.Listings, len(payoff),
				"the payoff listings must be attached whether the player won or lost")
			assert.Equal(t, "abc123", reveal.Listings[0].DisplayID)
			assert.Equal(t, "Угловой диван", reveal.Listings[0].Title)
			assert.Equal(t, int64(1500000), reveal.Listings[0].PriceKopeks)

			require.Len(t, listings.words, 1, "listings must be queried exactly once per round")
			assert.Equal(t, "диван", listings.words[0],
				"the pool must be queried with the normalized secret")
		})
	}
}

func TestListingsAreQueriedWithNormalizedSecret(t *testing.T) {
	listings := &mockListingPool{listings: []Listing{{DisplayID: "abc123", Title: "Ковёр"}}}

	game, round := newGame("ковёр", listings)

	_, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	_, err = game.Guess(context.Background(), round, json.RawMessage(`{"guess":"ковер"}`))
	require.NoError(t, err)

	require.Len(t, listings.words, 1)
	assert.Equal(t, "ковер", listings.words[0],
		"a yo-folded secret must reach the pool folded, otherwise the lookup misses every listing")
}

func TestTerminalRevealSurvivesListingFailure(t *testing.T) {
	tests := []struct {
		name string
		pool *mockListingPool
	}{
		{name: "pool errors", pool: &mockListingPool{err: assert.AnError}},
		{name: "pool returns nothing", pool: &mockListingPool{listings: nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game, round := newGame("диван", tt.pool)

			_, err := game.Start(context.Background(), round)
			require.NoError(t, err)

			outcome, err := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"диван"}`))
			require.NoError(t, err, "a listings failure must never break the round")

			assert.Equal(t, domain.ProgressWin, outcome.Progress)

			var reveal revealPayload
			require.NoError(t, json.Unmarshal(outcome.Reveal, &reveal))

			assert.True(t, reveal.GameOver)
			require.NotNil(t, reveal.Secret)
			assert.Equal(t, "диван", *reveal.Secret)
			assert.Empty(t, reveal.Listings)
		})
	}
}

func TestGuessRejectsCheating(t *testing.T) {
	tests := []struct {
		name  string
		move  string
		valid bool
	}{
		{name: "wrong length", move: `{"guess":"стол"}`, valid: true},
		{name: "latin letters", move: `{"guess":"house"}`, valid: true},
		{name: "digits", move: `{"guess":"12345"}`, valid: true},
		{name: "oversized", move: `{"guess":"чрезвычайнодлинноеслово"}`, valid: true},
		{name: "empty", move: `{"guess":""}`, valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game := New(&mockWordPool{word: "диван", valid: tt.valid}, nil)
			round := domain.NewRound(uuid.New(), Slug, time.Now())

			_, err := game.Start(context.Background(), round)
			require.NoError(t, err)

			_, err = game.Guess(context.Background(), round, json.RawMessage(tt.move))
			require.Error(t, err)
		})
	}
}

func TestGuessRejectsReplay(t *testing.T) {
	game, round := newGame("диван", nil)

	_, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	_, err = game.Guess(context.Background(), round, json.RawMessage(`{"guess":"книга"}`))
	require.NoError(t, err)

	_, err = game.Guess(context.Background(), round, json.RawMessage(`{"guess":"книга"}`))
	require.Error(t, err, "replaying the same guess must be rejected")
}

func TestGuessNormalizesCase(t *testing.T) {
	game, round := newGame("диван", nil)

	_, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	outcome, err := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"ДИВАН"}`))
	require.NoError(t, err)

	assert.Equal(t, domain.ProgressWin, outcome.Progress)
}

func TestGuessNormalizesYo(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		guess  string
	}{
		{name: "ye guess solves yo secret", secret: "ковёр", guess: "ковер"},
		{name: "yo guess solves ye secret", secret: "ковер", guess: "ковёр"},
		{name: "yo guess solves yo secret", secret: "ковёр", guess: "ковёр"},
		{name: "uppercase yo solves yo secret", secret: "ковёр", guess: "КОВЁР"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game, round := newGame(tt.secret, nil)

			_, err := game.Start(context.Background(), round)
			require.NoError(t, err)

			outcome, err := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"`+tt.guess+`"}`))
			require.NoError(t, err)

			assert.Equal(t, domain.ProgressWin, outcome.Progress,
				"yo and ye must be the same letter, otherwise the mismatch is an oracle")
		})
	}
}

func TestSecretIsStoredNormalized(t *testing.T) {
	game, round := newGame("ковёр", nil)

	_, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	assert.NotContains(t, string(round.Payload()), "ё",
		"the stored secret must already be yo-folded")
	assert.Contains(t, string(round.Payload()), "ковер")
}

func TestYoNormalizationIsNotAnOracle(t *testing.T) {
	game, round := newGame("ковёр", nil)

	_, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	outcome, err := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"кивер"}`))
	require.NoError(t, err)

	var reveal revealPayload
	require.NoError(t, json.Unmarshal(outcome.Reveal, &reveal))
	require.Len(t, reveal.Feedback, 5)

	assert.Equal(t, StatusCorrect, reveal.Feedback[3].Status,
		"the fourth letter must read as correct regardless of yo spelling")
}

func TestWrongGuessContinuesWithoutTouchingStreak(t *testing.T) {
	game, round := newGame("диван", nil)

	_, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	outcome, err := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"книга"}`))
	require.NoError(t, err)

	assert.False(t, outcome.Correct, "a wrong word is wrong, whatever the round does next")
	assert.Equal(t, domain.ProgressContinue, outcome.Progress)
	require.NotNil(t, outcome.Next, "the round stays active with tries left")

	assert.Equal(t, 0, round.Streak(), "bukovki must never move the streak itself")
	assert.Equal(t, 0, round.BestStreak())
	assert.True(t, round.IsActive())
	assert.Equal(t, 1, game.AttemptsUsed(round))
}

func TestSolvingOnThirdAttemptWins(t *testing.T) {
	game, round := newGame("диван", nil)

	_, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	for _, wrong := range []string{"книга", "лампа"} {
		outcome, guessErr := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"`+wrong+`"}`))
		require.NoError(t, guessErr)
		require.Equal(t, domain.ProgressContinue, outcome.Progress)
	}

	outcome, err := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"диван"}`))
	require.NoError(t, err)

	assert.True(t, outcome.Correct)
	assert.Equal(t, domain.ProgressWin, outcome.Progress)
	assert.Nil(t, outcome.Next)
	assert.Equal(t, 3, game.AttemptsUsed(round))
	assert.Equal(t, 0, round.Streak(), "winning must not fabricate a streak")
}

func TestExhaustingEveryAttemptLoses(t *testing.T) {
	game, round := newGame("диван", nil)

	_, err := game.Start(context.Background(), round)
	require.NoError(t, err)

	wrong := []string{"книга", "лампа", "комод", "чехол", "робот"}
	require.Len(t, wrong, maxTries-1)

	for i, word := range wrong {
		outcome, guessErr := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"`+word+`"}`))
		require.NoError(t, guessErr)
		require.Equal(t, domain.ProgressContinue, outcome.Progress)
		require.Equal(t, i+1, game.AttemptsUsed(round))
	}

	outcome, err := game.Guess(context.Background(), round, json.RawMessage(`{"guess":"санки"}`))
	require.NoError(t, err)

	assert.False(t, outcome.Correct)
	assert.Equal(t, domain.ProgressLose, outcome.Progress)
	assert.Nil(t, outcome.Next)
	assert.Equal(t, maxTries, game.AttemptsUsed(round))
}

func TestBukovkiIsNotStreakBased(t *testing.T) {
	game, _ := newGame("диван", nil)

	assert.Equal(t, 0, game.TargetStreak(), "bukovki does not use the streak target")
	assert.Equal(t, maxTries, game.MaxAttempts())
}

func TestNormalizeWordFoldsYo(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "ковёр", want: "ковер"},
		{in: "КОВЁР", want: "ковер"},
		{in: "  Щётка ", want: "щетка"},
		{in: "диван", want: "диван"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, NormalizeWord(tt.in))
	}
}

func TestFallbackWordsAreWithinPlayableRange(t *testing.T) {
	words := FallbackWords(minWordLength, maxWordLength)
	require.NotEmpty(t, words)

	for _, word := range words {
		length := RuneLen(word)
		assert.GreaterOrEqual(t, length, minWordLength)
		assert.LessOrEqual(t, length, maxWordLength)
		assert.True(t, IsCyrillicWord(word), "%q must be cyrillic", word)
		assert.Equal(t, strings.ToLower(word), word)
		assert.Equal(t, NormalizeWord(word), word,
			"%q must already be normalized, otherwise it is unsolvable", word)
	}
}
