package domain

import (
	"fmt"

	"github.com/hel1th/pet-economy/shared/domainerr"
)

var (
	ErrGameNotFound  = fmt.Errorf("game not found: %w", domainerr.ErrNotFound)
	ErrRoundNotFound = fmt.Errorf("round not found: %w", domainerr.ErrNotFound)
	ErrNoItemsInPool = fmt.Errorf("game item pool is empty: %w", domainerr.ErrNotFound)
)

func ErrRoundFinished(state State) error {
	return domainerr.NewConflict(fmt.Sprintf("round is already %s", state))
}

func ErrRewardNotReady() error {
	return domainerr.NewConflict("weekly reward is not available yet")
}

func ErrInvalidMove(message string) error {
	return domainerr.NewInvalid("move", message)
}
