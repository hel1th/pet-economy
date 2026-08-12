package domain

import "time"

const (
	checkInSatietyGain = 10
	maxFreezes         = 3
)

var streakMilestones = map[int]int{
	3:  30,
	7:  70,
	14: 150,
	30: 300,
}

type StreakOutcome struct {
	Days             int
	Continued        bool
	FreezeUsed       bool
	Reset            bool
	MilestoneBonus   int
	FreezesLeft      int
	MilestoneReached int
}

func (p *Pet) advanceStreak(now time.Time) StreakOutcome {
	today := day(now)
	outcome := StreakOutcome{}

	switch {
	case p.lastCheckInDate == nil:
		p.streakDays = 1
	case day(*p.lastCheckInDate).Equal(today.AddDate(0, 0, -1)):
		p.streakDays++
		outcome.Continued = true
	case p.canFreeze(today):
		p.freezes--
		p.streakDays++
		outcome.Continued = true
		outcome.FreezeUsed = true
	default:
		p.streakDays = 1
		outcome.Reset = true
	}

	p.lastCheckInDate = &today
	outcome.Days = p.streakDays
	outcome.FreezesLeft = p.freezes

	if bonus, ok := streakMilestones[p.streakDays]; ok {
		outcome.MilestoneBonus = bonus
		outcome.MilestoneReached = p.streakDays
		p.grantFreeze()
		outcome.FreezesLeft = p.freezes
	}

	return outcome
}

func (p *Pet) canFreeze(today time.Time) bool {
	if p.freezes <= 0 || p.lastCheckInDate == nil {
		return false
	}
	missed := day(*p.lastCheckInDate).AddDate(0, 0, 2)

	return missed.Equal(today)
}

func (p *Pet) grantFreeze() {
	if p.freezes < maxFreezes {
		p.freezes++
	}
}

func (p *Pet) GrantFreeze() {
	p.grantFreeze()
}
