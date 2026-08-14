package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPetState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		satiety   int
		happiness int
		energy    int
		want      State
	}{
		{name: "sleeping wins over everything", satiety: 100, happiness: 100, energy: 19, want: StateSleeping},
		{name: "sad when hungry", satiety: 29, happiness: 100, energy: 50, want: StateSad},
		{name: "happy when cheerful", satiety: 50, happiness: 70, energy: 50, want: StateHappy},
		{name: "neutral otherwise", satiety: 50, happiness: 69, energy: 50, want: StateNeutral},
		{name: "boundary satiety 30 is not sad", satiety: 30, happiness: 10, energy: 50, want: StateNeutral},
		{name: "boundary energy 20 is awake", satiety: 50, happiness: 10, energy: 20, want: StateNeutral},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pet := petWithParams(tt.satiety, tt.happiness, tt.energy)
			assert.Equal(t, tt.want, pet.State())
			assert.True(t, pet.State().Valid())
		})
	}
}

func TestXPMultipliers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		satiety   int
		happiness int
		amount    int
		want      int
	}{
		{name: "hungry halves award", satiety: 10, happiness: 10, amount: 4, want: 2},
		{name: "happy boosts award", satiety: 80, happiness: 80, amount: 4, want: 5},
		{name: "hungry and happy combine", satiety: 10, happiness: 80, amount: 8, want: 5},
		{name: "neutral keeps award", satiety: 50, happiness: 50, amount: 3, want: 3},
		{name: "small award never drops to zero", satiety: 10, happiness: 10, amount: 1, want: 1},
		{name: "zero award stays zero", satiety: 10, happiness: 10, amount: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pet := petWithParams(tt.satiety, tt.happiness, 100)
			assert.Equal(t, tt.want, pet.effectiveAward(tt.amount))
		})
	}
}
