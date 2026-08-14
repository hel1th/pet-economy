package bukovki

import (
	"fmt"

	"github.com/hel1th/pet-economy/shared/domainerr"
)

var ErrNoWordsInPool = fmt.Errorf("bukovki word pool is empty: %w", domainerr.ErrNotFound)
