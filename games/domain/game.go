package domain

import (
	"context"
	"encoding/json"
)

type View struct {
	Prompt json.RawMessage
}

type Progress string

const (
	ProgressContinue Progress = "continue"
	ProgressAdvance  Progress = "advance"
	ProgressWin      Progress = "win"
	ProgressLose     Progress = "lose"
)

type GuessOutcome struct {
	Correct  bool
	Progress Progress
	Reveal   json.RawMessage
	Next     *View
}

type Game interface {
	Slug() string
	TargetStreak() int
	Start(ctx context.Context, r *Round) (View, error)
	Guess(ctx context.Context, r *Round, move json.RawMessage) (GuessOutcome, error)
	Resume(ctx context.Context, r *Round) (View, error)
}

type AttemptLimited interface {
	MaxAttempts() int
	AttemptsUsed(r *Round) int
}

type Scored interface {
	RoundScore(r *Round) int
	CountsTowardStreak(score int) bool
}

func ScoredOf(game Game) (Scored, bool) {
	scored, ok := game.(Scored)

	return scored, ok
}

func MaxAttemptsOf(game Game) int {
	limited, ok := game.(AttemptLimited)
	if !ok {
		return 0
	}

	return limited.MaxAttempts()
}

func AttemptsUsedOf(game Game, r *Round) int {
	limited, ok := game.(AttemptLimited)
	if !ok || r == nil {
		return 0
	}

	return limited.AttemptsUsed(r)
}

type Registry struct {
	games map[string]Game
	order []string
}

func NewRegistry(games ...Game) *Registry {
	registry := &Registry{games: make(map[string]Game, len(games))}

	for _, game := range games {
		registry.Register(game)
	}

	return registry
}

func (r *Registry) Register(game Game) {
	if game == nil {
		return
	}

	slug := game.Slug()
	if slug == "" {
		return
	}

	if _, exists := r.games[slug]; !exists {
		r.order = append(r.order, slug)
	}

	r.games[slug] = game
}

func (r *Registry) Get(slug string) (Game, error) {
	game, ok := r.games[slug]
	if !ok {
		return nil, ErrGameNotFound
	}

	return game, nil
}

func (r *Registry) All() []Game {
	out := make([]Game, 0, len(r.order))

	for _, slug := range r.order {
		out = append(out, r.games[slug])
	}

	return out
}

func (r *Registry) Slugs() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)

	return out
}
