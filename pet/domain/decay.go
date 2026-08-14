package domain

import (
	"math"
	"time"
)

const (
	satietyRatePerHour   = -2.0
	happinessRatePerHour = -1.5
	energyRatePerHour    = 5.0
	minParameterValue    = 0
	maxDecayHours        = 24 * 14
)

func (p *Pet) ApplyDecay(now time.Time) {
	elapsed := now.Sub(p.lastDecayTime)
	if elapsed <= 0 {
		return
	}

	hours := elapsed.Hours()
	if hours > maxDecayHours {
		hours = maxDecayHours
	}

	p.satiety = shiftParameter(p.satiety, satietyRatePerHour*hours)
	p.happiness = shiftParameter(p.happiness, happinessRatePerHour*hours)
	p.energy = shiftParameter(p.energy, energyRatePerHour*hours)
	p.lastDecayTime = now
}

func (p *Pet) Feed(amount int, now time.Time) {
	p.ApplyDecay(now)
	p.satiety = shiftParameter(p.satiety, float64(amount))
	p.updatedAt = now
}

func (p *Pet) Cheer(amount int, now time.Time) {
	p.ApplyDecay(now)
	p.happiness = shiftParameter(p.happiness, float64(amount))
	p.updatedAt = now
}

func shiftParameter(current int, delta float64) int {
	return clampParameter(int(math.Round(float64(current) + delta)))
}

func clampParameter(value int) int {
	if value < minParameterValue {
		return minParameterValue
	}
	if value > maxParameterValue {
		return maxParameterValue
	}

	return value
}
