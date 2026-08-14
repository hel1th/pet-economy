package domain

import "time"

const RewardTargetDays = 7

type Day struct {
	Year  int
	Month time.Month
	Day   int
}

func DayOf(t time.Time) Day {
	utc := t.UTC()
	year, month, day := utc.Date()

	return Day{Year: year, Month: month, Day: day}
}

func (d Day) Time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func (d Day) AddDays(n int) Day {
	return DayOf(d.Time().AddDate(0, 0, n))
}

func (d Day) Equal(other Day) bool {
	return d == other
}

func (d Day) IsDayBefore(other Day) bool {
	return d.AddDays(1).Equal(other)
}

type Streak struct {
	CurrentDays     int
	BestDays        int
	LastDay         *Day
	RewardClaimedAt *time.Time
}

func AdvanceStreak(s Streak, today Day) Streak {
	switch {
	case s.LastDay != nil && s.LastDay.Equal(today):
		return s
	case s.LastDay != nil && s.LastDay.IsDayBefore(today):
		s.CurrentDays++
	default:
		s.CurrentDays = 1
	}

	if s.CurrentDays > s.BestDays {
		s.BestDays = s.CurrentDays
	}

	day := today
	s.LastDay = &day

	return s
}

func RewardReady(s Streak) bool {
	return s.CurrentDays >= RewardTargetDays
}

func ClaimStreak(s Streak, now time.Time) (Streak, bool) {
	if !RewardReady(s) {
		return s, false
	}

	claimed := now.UTC()
	s.RewardClaimedAt = &claimed
	s.CurrentDays = 0

	return s, true
}

type DailyProgress struct {
	Attempts   int
	BestStreak int
}

func AdvanceDaily(p DailyProgress, roundStreak int) DailyProgress {
	p.Attempts++

	if roundStreak > p.BestStreak {
		p.BestStreak = roundStreak
	}

	return p
}

func IsFirstAttemptOfDay(p DailyProgress) bool {
	return p.Attempts == 0
}
