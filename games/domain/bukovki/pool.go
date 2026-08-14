package bukovki

import (
	"context"
	"math/rand"
	"strings"
	"unicode"
)

const (
	minWordLength = 4
	maxWordLength = 8
)

type Listing struct {
	DisplayID   string
	Title       string
	PhotoURL    string
	PriceKopeks int64
}

type WordPool interface {
	RandomWord(ctx context.Context, minLength, maxLength int) (string, error)
	IsValidWord(ctx context.Context, word string, length int) bool
}

type ListingPool interface {
	ListingsByWord(ctx context.Context, word string, limit int) ([]Listing, error)
}

var fallbackWords = []string{
	"стол", "полка", "диван", "книга", "лампа", "комод",
	"чехол", "робот", "санки", "набор", "плита", "халат",
	"кофта", "шарик", "петля", "туфли", "ковер", "гитара",
	"кресло", "шкатулка", "коляска", "самокат", "монитор", "рюкзак",
}

type FallbackWordPool struct{}

func NewFallbackWordPool() *FallbackWordPool {
	return &FallbackWordPool{}
}

func (p *FallbackWordPool) RandomWord(_ context.Context, minLength, maxLength int) (string, error) {
	words := FallbackWords(minLength, maxLength)
	if len(words) == 0 {
		return "", ErrNoWordsInPool
	}

	return words[rand.Intn(len(words))], nil
}

func (p *FallbackWordPool) IsValidWord(_ context.Context, word string, length int) bool {
	normalized := NormalizeWord(word)

	return RuneLen(normalized) == length && IsCyrillicWord(normalized)
}

func FallbackWords(minLength, maxLength int) []string {
	out := make([]string, 0, len(fallbackWords))

	for _, word := range fallbackWords {
		length := len([]rune(word))
		if length >= minLength && length <= maxLength {
			out = append(out, word)
		}
	}

	return out
}

func NormalizeWord(word string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(word)), "ё", "е")
}

func RuneLen(word string) int {
	return len([]rune(word))
}

func IsCyrillicWord(word string) bool {
	if word == "" {
		return false
	}

	for _, r := range word {
		if !unicode.Is(unicode.Cyrillic, r) {
			return false
		}
	}

	return true
}
