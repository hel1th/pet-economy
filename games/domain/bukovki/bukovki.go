package bukovki

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hel1th/pet-economy/games/domain"
)

const Slug = "bukovki"

const (
	maxTries          = 6
	maxRevealListings = 4
)

type Game struct {
	pool     WordPool
	listings ListingPool
}

func New(pool WordPool, listings ListingPool) *Game {
	return &Game{pool: pool, listings: listings}
}

func (g *Game) Slug() string { return Slug }

func (g *Game) TargetStreak() int { return 0 }

func (g *Game) MaxAttempts() int { return maxTries }

func (g *Game) AttemptsUsed(r *domain.Round) int {
	s, err := decodeState(r.Payload())
	if err != nil {
		return 0
	}

	return len(s.Guesses)
}

func (g *Game) Start(ctx context.Context, r *domain.Round) (domain.View, error) {
	secretWord, err := g.pool.RandomWord(ctx, minWordLength, maxWordLength)
	if err != nil {
		return domain.View{}, err
	}

	secret := NormalizeWord(secretWord)

	length := RuneLen(secret)
	if length < minWordLength || length > maxWordLength || !IsCyrillicWord(secret) {
		return domain.View{}, ErrNoWordsInPool
	}

	state := state{
		Secret:   secret,
		Guesses:  make([]string, 0),
		MaxTries: maxTries,
	}

	return g.commit(r, state)
}

func (g *Game) Resume(ctx context.Context, r *domain.Round) (domain.View, error) {
	s, err := decodeState(r.Payload())
	if err != nil {
		return domain.View{}, err
	}

	prompt, err := s.prompt()
	if err != nil {
		return domain.View{}, err
	}
	return domain.View{Prompt: prompt}, nil
}

func (g *Game) Guess(ctx context.Context, r *domain.Round, move json.RawMessage) (domain.GuessOutcome, error) {
	wordGuess, err := parseMove(move)
	if err != nil {
		return domain.GuessOutcome{}, err
	}

	s, err := decodeState(r.Payload())
	if err != nil {
		return domain.GuessOutcome{}, domain.ErrInvalidMove("failed to decode state")
	}

	secretLength := RuneLen(s.Secret)

	if RuneLen(wordGuess) != secretLength {
		return domain.GuessOutcome{}, domain.ErrInvalidMove("guess must match the word length")
	}

	if alreadyGuessed(s.Guesses, wordGuess) {
		return domain.GuessOutcome{}, domain.ErrInvalidMove("word was already guessed")
	}

	if !g.pool.IsValidWord(ctx, wordGuess, secretLength) {
		return domain.GuessOutcome{}, domain.ErrInvalidMove("word is not valid")
	}

	s.Guesses = append(s.Guesses, wordGuess)

	lettersFeedback := evaluateGuess(wordGuess, s.Secret)
	win := isCorrect(lettersFeedback)
	gameOver := win || len(s.Guesses) == s.MaxTries

	reveal := revealPayload{
		Feedback: lettersFeedback,
		GameOver: gameOver,
		Win:      win,
	}
	if gameOver {
		reveal.Secret = &s.Secret
		reveal.Listings = toListingPayloads(g.matchingListings(ctx, s.Secret))
	}

	revealJSON, err := json.Marshal(reveal)
	if err != nil {
		return domain.GuessOutcome{}, err
	}

	if win {
		if _, err := g.commit(r, s); err != nil {
			return domain.GuessOutcome{}, err
		}
		return domain.GuessOutcome{Correct: true, Progress: domain.ProgressWin, Reveal: revealJSON}, nil
	}

	if gameOver {
		if _, err := g.commit(r, s); err != nil {
			return domain.GuessOutcome{}, err
		}
		return domain.GuessOutcome{Correct: false, Progress: domain.ProgressLose, Reveal: revealJSON}, nil
	}

	view, err := g.commit(r, s)
	if err != nil {
		return domain.GuessOutcome{}, err
	}
	return domain.GuessOutcome{Correct: false, Progress: domain.ProgressContinue, Reveal: revealJSON, Next: &view}, nil
}

func alreadyGuessed(guesses []string, word string) bool {
	for _, guess := range guesses {
		if guess == word {
			return true
		}
	}

	return false
}

func (g *Game) matchingListings(ctx context.Context, secret string) []Listing {
	if g.listings == nil {
		return nil
	}

	listings, err := g.listings.ListingsByWord(ctx, secret, maxRevealListings)
	if err != nil {
		return nil
	}

	return listings
}

func isCorrect(feedback []LetterFeedback) bool {
	for _, f := range feedback {
		if f.Status != StatusCorrect {
			return false
		}
	}
	return true
}

func (g *Game) commit(r *domain.Round, s state) (domain.View, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return domain.View{}, fmt.Errorf("marshal bukovki state: %w", err)
	}

	prompt, err := s.prompt()
	if err != nil {
		return domain.View{}, err
	}

	r.SetPayload(payload)

	return domain.View{Prompt: prompt}, nil
}

func parseMove(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", domain.ErrInvalidMove("move is required")
	}

	var move movePayload
	if err := json.Unmarshal(raw, &move); err != nil {
		return "", domain.ErrInvalidMove("move must be an object with a guess field")
	}

	guess := NormalizeWord(move.Guess)

	if guess == "" {
		return "", domain.ErrInvalidMove("guess is required")
	}

	if RuneLen(guess) > maxWordLength {
		return "", domain.ErrInvalidMove("guess is too long")
	}

	if !IsCyrillicWord(guess) {
		return "", domain.ErrInvalidMove("guess must contain cyrillic letters only")
	}

	return guess, nil
}

func evaluateGuess(guess, secret string) []LetterFeedback {
	guessRunes := []rune(guess)
	secretRunes := []rune(secret)

	secretCounts := make(map[rune]int)
	feedback := make([]LetterFeedback, len(guessRunes))

	for _, r := range secretRunes {
		secretCounts[r]++
	}

	for i, r := range guessRunes {
		if i < len(secretRunes) && r == secretRunes[i] {
			feedback[i] = LetterFeedback{Char: string(r), Status: StatusCorrect}
			secretCounts[r]--
		}
	}

	for i, r := range guessRunes {
		if feedback[i].Status == StatusCorrect {
			continue
		}

		if count, ok := secretCounts[r]; ok && count > 0 {
			feedback[i] = LetterFeedback{Char: string(r), Status: StatusPresent}
			secretCounts[r]--
		} else {
			feedback[i] = LetterFeedback{Char: string(r), Status: StatusAbsent}
		}
	}

	return feedback
}
