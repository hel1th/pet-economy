package domain

import "math"

type State string

const (
	StateHappy    State = "happy"
	StateNeutral  State = "neutral"
	StateSad      State = "sad"
	StateSleeping State = "sleeping"
)

const (
	sadSatietyThreshold      = 30
	happyHappinessThreshold  = 70
	sleepingEnergyThreshold  = 20
	hungryXPMultiplier       = 0.5
	happyXPMultiplier        = 1.25
	neutralXPMultiplier      = 1.0
	minimalAwardWhenPositive = 1
)

func (s State) Valid() bool {
	switch s {
	case StateHappy, StateNeutral, StateSad, StateSleeping:
		return true
	default:
		return false
	}
}

func (p *Pet) State() State {
	switch {
	case p.energy < sleepingEnergyThreshold:
		return StateSleeping
	case p.satiety < sadSatietyThreshold:
		return StateSad
	case p.happiness >= happyHappinessThreshold:
		return StateHappy
	default:
		return StateNeutral
	}
}

func (p *Pet) xpMultiplier() float64 {
	multiplier := neutralXPMultiplier
	if p.satiety < sadSatietyThreshold {
		multiplier *= hungryXPMultiplier
	}
	if p.happiness >= happyHappinessThreshold {
		multiplier *= happyXPMultiplier
	}

	return multiplier
}

func (p *Pet) effectiveAward(amount int) int {
	if amount <= 0 {
		return amount
	}

	scaled := int(math.Round(float64(amount) * p.xpMultiplier()))
	if scaled < minimalAwardWhenPositive {
		return minimalAwardWhenPositive
	}

	return scaled
}
