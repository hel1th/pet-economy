package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
)

type ProgressView struct {
	Level       int
	XP          int
	NextLevelXP int
	XPToNext    int
	IsMaxLevel  bool
	Stage       domain.Stage
	StreakDays  int
	Freezes     int
}

func (s *Service) Progress(ctx context.Context, userID uuid.UUID) (ProgressView, error) {
	pet, err := s.State(ctx, userID)
	if err != nil {
		return ProgressView{}, err
	}

	toNext := pet.NextLevelXP() - pet.XP()
	if toNext < 0 || pet.IsMaxLevel() {
		toNext = 0
	}

	return ProgressView{
		Level:       pet.Level(),
		XP:          pet.XP(),
		NextLevelXP: pet.NextLevelXP(),
		XPToNext:    toNext,
		IsMaxLevel:  pet.IsMaxLevel(),
		Stage:       pet.Stage(),
		StreakDays:  pet.StreakDays(),
		Freezes:     pet.Freezes(),
	}, nil
}
