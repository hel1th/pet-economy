package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	DefaultName = "Ноти"

	initialLevel     = 1
	initialSatiety   = 70
	initialHappiness = 70
	initialEnergy    = 100
	initialFreezes   = 0

	MaxParameterValue   = 100
	StrokeHappinessGain = 5
	FeedSatietyGain     = 15

	maxParameterValue   = MaxParameterValue
	strokeHappinessGain = StrokeHappinessGain
	feedSatietyGain     = FeedSatietyGain
)

type Pet struct {
	id              uuid.UUID
	userID          uuid.UUID
	name            string
	stage           Stage
	level           int
	xp              int
	nextLevelXP     int
	satiety         int
	happiness       int
	energy          int
	streakDays      int
	freezes         int
	lastCheckInDate *time.Time
	hatchedAt       *time.Time
	lastDecayTime   time.Time
	updatedAt       time.Time

	interactionVersion int64
}

func New(userID uuid.UUID, now time.Time) *Pet {
	hatched := now

	return &Pet{
		id:            uuid.New(),
		userID:        userID,
		name:          DefaultName,
		stage:         stageForLevel(initialLevel),
		level:         initialLevel,
		nextLevelXP:   nextThreshold(initialLevel),
		satiety:       initialSatiety,
		happiness:     initialHappiness,
		energy:        initialEnergy,
		freezes:       initialFreezes,
		hatchedAt:     &hatched,
		lastDecayTime: now,
		updatedAt:     now,
	}
}

type RestoreParams struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Name            string
	Stage           Stage
	Level           int
	XP              int
	NextLevelXP     int
	Satiety         int
	Happiness       int
	Energy          int
	StreakDays      int
	Freezes         int
	LastCheckInDate *time.Time
	HatchedAt       *time.Time
	LastDecayTime   time.Time
	UpdatedAt       time.Time

	InteractionVersion int64
}

func Restore(p RestoreParams) *Pet {
	return &Pet{
		id: p.ID, userID: p.UserID, name: p.Name, stage: p.Stage, level: p.Level, xp: p.XP,
		nextLevelXP: nextThreshold(p.Level), satiety: p.Satiety, happiness: p.Happiness,
		energy: p.Energy, streakDays: p.StreakDays, freezes: p.Freezes,
		lastCheckInDate: cloneTime(p.LastCheckInDate), hatchedAt: cloneTime(p.HatchedAt),
		lastDecayTime: p.LastDecayTime, updatedAt: p.UpdatedAt,
		interactionVersion: p.InteractionVersion,
	}
}

func (p *Pet) Stroke(now time.Time) {
	p.happiness = min(p.happiness+strokeHappinessGain, maxParameterValue)
	p.interactionVersion++
	p.updatedAt = now
}

func (p *Pet) FeedMeal(now time.Time) {
	p.satiety = min(p.satiety+feedSatietyGain, maxParameterValue)
	p.interactionVersion++
	p.updatedAt = now
}

func (p *Pet) InteractionVersion() int64 { return p.interactionVersion }

func (p *Pet) ApplyHotState(happiness, satiety int, version int64, updatedAt time.Time) error {
	if happiness < 0 || happiness > maxParameterValue ||
		satiety < 0 || satiety > maxParameterValue || version < 0 {
		return ErrInvalidAction
	}
	if version < p.interactionVersion {
		return ErrInvalidAction
	}

	p.happiness = happiness
	p.satiety = satiety
	p.interactionVersion = version
	if updatedAt.After(p.updatedAt) {
		p.updatedAt = updatedAt
	}

	return nil
}

func (p *Pet) IsHatched() bool { return p.hatchedAt != nil }

func (p *Pet) Hatch(now time.Time) bool {
	if p.hatchedAt != nil {
		return false
	}

	hatched := now
	p.hatchedAt = &hatched
	p.stage = stageForLevel(p.level)
	p.updatedAt = now

	return true
}

func (p *Pet) ID() uuid.UUID     { return p.id }
func (p *Pet) UserID() uuid.UUID { return p.userID }

func (p *Pet) Name() string {
	if p.name == "" {
		return DefaultName
	}

	return p.name
}

func (p *Pet) Stage() Stage                { return p.stage }
func (p *Pet) Level() int                  { return p.level }
func (p *Pet) XP() int                     { return p.xp }
func (p *Pet) NextLevelXP() int            { return p.nextLevelXP }
func (p *Pet) Satiety() int                { return p.satiety }
func (p *Pet) Happiness() int              { return p.happiness }
func (p *Pet) Energy() int                 { return p.energy }
func (p *Pet) StreakDays() int             { return p.streakDays }
func (p *Pet) Freezes() int                { return p.freezes }
func (p *Pet) IsMaxLevel() bool            { return p.level >= MaxLevel }
func (p *Pet) LastCheckInDate() *time.Time { return cloneTime(p.lastCheckInDate) }
func (p *Pet) HatchedAt() *time.Time       { return cloneTime(p.hatchedAt) }
func (p *Pet) LastDecayTime() time.Time    { return p.lastDecayTime }
func (p *Pet) UpdatedAt() time.Time        { return p.updatedAt }

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
