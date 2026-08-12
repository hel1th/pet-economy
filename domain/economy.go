package domain

import (
	"math"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	dailyLoginXP          = 1
	favoriteXP            = 1
	qualityListingXP      = 2
	itemViewedXP          = 1
	qualityDescriptionLen = 200
	moscowOffsetSeconds   = 3 * 60 * 60
	streakBonusDays       = 7
	streakBonusFactor     = 1.5
)

const (
	MaxFavoritesPerDay = 5
	MaxItemViewsPerDay = 10
)

var moscowZone = time.FixedZone("MSK", moscowOffsetSeconds)

type LimitedAction struct {
	SubjectID     uuid.UUID
	Unique        bool
	RewardedCount int
	Now           time.Time
}

type QualityListing struct {
	HasPhoto    bool
	HasPrice    bool
	Description string
	Now         time.Time
}

func (p *Pet) DailyCheckIn(now time.Time) (Progress, error) {
	outcome, err := p.CheckIn(now)
	if err != nil {
		return Progress{}, err
	}

	return outcome.Progress, nil
}

type CheckInResult struct {
	Progress Progress
	Streak   StreakOutcome
}

func (p *Pet) CheckIn(now time.Time) (CheckInResult, error) {
	today := day(now)
	if p.lastCheckInDate != nil && day(*p.lastCheckInDate).Equal(today) {
		return CheckInResult{}, ErrDuplicateAction
	}

	p.ApplyDecay(now)
	streak := p.advanceStreak(now)

	amount := dailyLoginXP
	if p.streakDays >= streakBonusDays {
		amount = int(math.Round(float64(dailyLoginXP) * streakBonusFactor))
	}
	amount += streak.MilestoneBonus

	p.satiety = shiftParameter(p.satiety, checkInSatietyGain)

	return CheckInResult{Progress: p.applyAward(amount, now), Streak: streak}, nil
}

func (p *Pet) RewardFavorite(action LimitedAction) (Progress, error) {
	return p.rewardLimited(action, MaxFavoritesPerDay, favoriteXP)
}

func (p *Pet) RewardItemViewed(action LimitedAction) (Progress, error) {
	return p.rewardLimited(action, MaxItemViewsPerDay, itemViewedXP)
}

func (p *Pet) RewardQualityListing(listing QualityListing) (Progress, error) {
	if !listing.HasPhoto || !listing.HasPrice || !hasQualityDescription(listing.Description) {
		return Progress{}, ErrConditionNotMet
	}

	return p.applyAward(qualityListingXP, listing.Now), nil
}

func hasQualityDescription(description string) bool {
	return utf8.RuneCountInString(description) > qualityDescriptionLen
}

func (p *Pet) rewardLimited(action LimitedAction, limit, amount int) (Progress, error) {
	if action.SubjectID == uuid.Nil {
		return Progress{}, ErrInvalidAction
	}
	if action.RewardedCount < 0 {
		return Progress{}, ErrInvalidAction
	}
	if !action.Unique {
		return Progress{}, ErrDuplicateAction
	}
	if action.RewardedCount >= limit {
		return Progress{}, ErrLimitReached
	}

	return p.applyAward(amount, action.Now), nil
}

func (p *Pet) applyAward(amount int, now time.Time) Progress {
	p.ApplyDecay(now)
	p.updatedAt = now

	return p.awardXP(p.effectiveAward(amount))
}

func day(value time.Time) time.Time {
	local := value.In(moscowZone)
	year, month, date := local.Date()

	return time.Date(year, month, date, 0, 0, 0, 0, moscowZone)
}
