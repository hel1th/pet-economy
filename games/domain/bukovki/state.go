package bukovki

import (
	"encoding/json"
	"fmt"

	"github.com/hel1th/pet-economy/games/domain"
)

type state struct {
	Secret   string   `json:"secret"`
	Guesses  []string `json:"guesses"`
	MaxTries int      `json:"max_tries"`
}

type movePayload struct {
	Guess string `json:"guess"`
}

type LetterStatus string

const (
	StatusCorrect LetterStatus = "correct"
	StatusPresent LetterStatus = "present"
	StatusAbsent  LetterStatus = "absent"
)

type LetterFeedback struct {
	Char   string       `json:"char"`
	Status LetterStatus `json:"status"`
}

func (f LetterFeedback) String() string {
	return string(f.Status)
}

type promptPayload struct {
	WordLength int                `json:"word_length"`
	MaxTries   int                `json:"max_tries"`
	History    [][]LetterFeedback `json:"history"`
}

type listingPayload struct {
	DisplayID   string `json:"display_id"`
	Title       string `json:"title"`
	PriceKopeks int64  `json:"price_kopeks"`
	PhotoURL    string `json:"photo_url"`
}

type revealPayload struct {
	Feedback []LetterFeedback `json:"feedback"`
	GameOver bool             `json:"game_over"`
	Win      bool             `json:"win"`
	Secret   *string          `json:"secret,omitempty"`
	Listings []listingPayload `json:"listings,omitempty"`
}

func toListingPayloads(listings []Listing) []listingPayload {
	out := make([]listingPayload, 0, len(listings))

	for _, listing := range listings {
		out = append(out, listingPayload{
			DisplayID:   listing.DisplayID,
			Title:       listing.Title,
			PriceKopeks: listing.PriceKopeks,
			PhotoURL:    listing.PhotoURL,
		})
	}

	return out
}

func decodeState(raw json.RawMessage) (state, error) {
	var s state
	if err := json.Unmarshal(raw, &s); err != nil {
		return state{}, fmt.Errorf("decode bukovki state: %w", err)
	}

	if s.Secret == "" {
		return state{}, domain.ErrRoundNotFound
	}

	return s, nil
}

func (s state) prompt() (json.RawMessage, error) {
	history := make([][]LetterFeedback, 0, len(s.Guesses))
	for _, guess := range s.Guesses {
		history = append(history, evaluateGuess(guess, s.Secret))
	}

	payload := promptPayload{
		WordLength: len([]rune(s.Secret)),
		MaxTries:   s.MaxTries,
		History:    history,
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal bukovki prompt: %w", err)
	}

	return encoded, nil
}
