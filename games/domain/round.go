package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/shared/publicid"
)

type State string

const (
	StateActive State = "active"
	StateWon    State = "won"
	StateLost   State = "lost"
)

func (s State) Valid() bool {
	switch s {
	case StateActive, StateWon, StateLost:
		return true
	default:
		return false
	}
}

func (s State) String() string {
	return string(s)
}

type Round struct {
	id         uuid.UUID
	displayID  string
	userID     uuid.UUID
	gameSlug   string
	state      State
	streak     int
	bestStreak int
	payload    json.RawMessage
	createdAt  time.Time
	updatedAt  time.Time
}

func NewRound(userID uuid.UUID, gameSlug string, now time.Time) *Round {
	return &Round{
		id:        uuid.New(),
		displayID: publicid.New(),
		userID:    userID,
		gameSlug:  gameSlug,
		state:     StateActive,
		payload:   json.RawMessage("{}"),
		createdAt: now,
		updatedAt: now,
	}
}

func RestoreRound(
	id uuid.UUID,
	displayID string,
	userID uuid.UUID,
	gameSlug string,
	state State,
	streak, bestStreak int,
	payload json.RawMessage,
	createdAt, updatedAt time.Time,
) *Round {
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}

	return &Round{
		id:         id,
		displayID:  displayID,
		userID:     userID,
		gameSlug:   gameSlug,
		state:      state,
		streak:     streak,
		bestStreak: bestStreak,
		payload:    payload,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

func (r *Round) ID() uuid.UUID            { return r.id }
func (r *Round) DisplayID() string        { return r.displayID }
func (r *Round) UserID() uuid.UUID        { return r.userID }
func (r *Round) GameSlug() string         { return r.gameSlug }
func (r *Round) State() State             { return r.state }
func (r *Round) Streak() int              { return r.streak }
func (r *Round) BestStreak() int          { return r.bestStreak }
func (r *Round) Payload() json.RawMessage { return r.payload }
func (r *Round) CreatedAt() time.Time     { return r.createdAt }
func (r *Round) UpdatedAt() time.Time     { return r.updatedAt }

func (r *Round) IsActive() bool {
	return r.state == StateActive
}

func (r *Round) SetPayload(payload json.RawMessage) {
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}

	r.payload = payload
}

func (r *Round) Advance(targetStreak int, now time.Time) {
	r.bump()

	if targetStreak > 0 && r.streak >= targetStreak {
		r.state = StateWon
	}

	r.updatedAt = now
}

func (r *Round) Win(now time.Time) {
	r.state = StateWon
	r.updatedAt = now
}

func (r *Round) bump() {
	r.streak++
	if r.streak > r.bestStreak {
		r.bestStreak = r.streak
	}
}

func (r *Round) Lose(now time.Time) {
	r.state = StateLost
	r.updatedAt = now
}

func (r *Round) Touch(now time.Time) {
	r.updatedAt = now
}
