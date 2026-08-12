package domain

import "errors"

var (
	ErrDuplicateAction = errors.New("action has already been rewarded")
	ErrLimitReached    = errors.New("action reward limit reached")
	ErrConditionNotMet = errors.New("reward condition is not met")
	ErrInvalidAction   = errors.New("invalid action data")

	ErrInvalidReward          = errors.New("invalid reward data")
	ErrRewardNotGranted       = errors.New("reward is not granted to the user")
	ErrRewardAlreadyActivated = errors.New("reward has already been activated")
	ErrRewardNotActivatable   = errors.New("reward cannot be activated")
	ErrRewardExpired          = errors.New("reward has expired")
	ErrRewardLocked           = errors.New("reward condition is not met")
)
