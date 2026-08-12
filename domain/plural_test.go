package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluralizeDaysCoversRussianForms(t *testing.T) {
	t.Parallel()

	tests := map[int]string{
		1:   "1 день",
		2:   "2 дня",
		4:   "4 дня",
		5:   "5 дней",
		11:  "11 дней",
		12:  "12 дней",
		14:  "14 дней",
		21:  "21 день",
		22:  "22 дня",
		25:  "25 дней",
		41:  "41 день",
		100: "100 дней",
		101: "101 день",
		111: "111 дней",
	}

	for count, expected := range tests {
		assert.Equal(t, expected, pluralize(count, daysForms))
	}
}

func TestPluralizeHandlesNegativeAndZero(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "0 дней", pluralize(0, daysForms))
	assert.Equal(t, "-1 день", pluralize(-1, daysForms))
	assert.Equal(t, "-3 дня", pluralize(-3, daysForms))
}

func TestTerminateKeepsStrongPunctuation(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Хороший вышел день!", terminate("Хороший вышел день!"))
	assert.Equal(t, "Куда пропал?", terminate("Куда пропал?"))
	assert.Equal(t, "День удался.", terminate("День удался."))
	assert.Equal(t, "День удался.", terminate("День удался"))
}
