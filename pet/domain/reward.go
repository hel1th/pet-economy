package domain

import (
	"time"

	"github.com/google/uuid"
)

type RewardKind string

const (
	RewardKindPromo    RewardKind = "promo"
	RewardKindUtility  RewardKind = "utility"
	RewardKindCosmetic RewardKind = "cosmetic"
)

func (k RewardKind) Valid() bool {
	switch k {
	case RewardKindPromo, RewardKindUtility, RewardKindCosmetic:
		return true
	default:
		return false
	}
}

type ConditionType string

const (
	ConditionLevel       ConditionType = "level"
	ConditionStreak      ConditionType = "streak"
	ConditionAchievement ConditionType = "achievement"
)

func (c ConditionType) Valid() bool {
	switch c {
	case ConditionLevel, ConditionStreak, ConditionAchievement:
		return true
	default:
		return false
	}
}

type RewardStatus string

const (
	RewardGranted   RewardStatus = "granted"
	RewardActivated RewardStatus = "activated"
	RewardExpired   RewardStatus = "expired"
)

func (s RewardStatus) Valid() bool {
	switch s {
	case RewardGranted, RewardActivated, RewardExpired:
		return true
	default:
		return false
	}
}

type Reward struct {
	id             string
	title          string
	description    string
	kind           RewardKind
	conditionType  ConditionType
	conditionValue int
}

type RewardParams struct {
	ID             string
	Title          string
	Description    string
	Kind           RewardKind
	ConditionType  ConditionType
	ConditionValue int
}

func NewReward(p RewardParams) (Reward, error) {
	if p.ID == "" {
		return Reward{}, ErrInvalidReward
	}
	if !p.Kind.Valid() || !p.ConditionType.Valid() || p.ConditionValue < 0 {
		return Reward{}, ErrInvalidReward
	}

	return Reward{
		id: p.ID, title: p.Title, description: p.Description, kind: p.Kind,
		conditionType: p.ConditionType, conditionValue: p.ConditionValue,
	}, nil
}

func (r Reward) ID() string                   { return r.id }
func (r Reward) Title() string                { return r.title }
func (r Reward) Description() string          { return r.description }
func (r Reward) Kind() RewardKind             { return r.kind }
func (r Reward) ConditionType() ConditionType { return r.conditionType }
func (r Reward) ConditionValue() int          { return r.conditionValue }

type UserReward struct {
	id          uuid.UUID
	userID      uuid.UUID
	rewardID    string
	status      RewardStatus
	code        string
	grantedAt   time.Time
	activatedAt *time.Time
	expiresAt   *time.Time
}

type UserRewardParams struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	RewardID    string
	Status      RewardStatus
	Code        string
	GrantedAt   time.Time
	ActivatedAt *time.Time
	ExpiresAt   *time.Time
}

func RestoreUserReward(p UserRewardParams) UserReward {
	return UserReward{
		id: p.ID, userID: p.UserID, rewardID: p.RewardID, status: p.Status, code: p.Code,
		grantedAt: p.GrantedAt, activatedAt: cloneTime(p.ActivatedAt), expiresAt: cloneTime(p.ExpiresAt),
	}
}

func GrantReward(userID uuid.UUID, rewardID string, now time.Time) (UserReward, error) {
	if userID == uuid.Nil || rewardID == "" {
		return UserReward{}, ErrInvalidReward
	}

	return UserReward{
		id: uuid.New(), userID: userID, rewardID: rewardID,
		status: RewardGranted, grantedAt: now,
	}, nil
}

func (u UserReward) ID() uuid.UUID           { return u.id }
func (u UserReward) UserID() uuid.UUID       { return u.userID }
func (u UserReward) RewardID() string        { return u.rewardID }
func (u UserReward) Status() RewardStatus    { return u.status }
func (u UserReward) Code() string            { return u.code }
func (u UserReward) GrantedAt() time.Time    { return u.grantedAt }
func (u UserReward) ActivatedAt() *time.Time { return cloneTime(u.activatedAt) }
func (u UserReward) ExpiresAt() *time.Time   { return cloneTime(u.expiresAt) }

func (u UserReward) IsActivated() bool { return u.status == RewardActivated }

func (u *UserReward) Activate(code string, now time.Time) error {
	if u.status == RewardActivated {
		return ErrRewardAlreadyActivated
	}
	if u.status != RewardGranted {
		return ErrRewardNotActivatable
	}
	if u.expiresAt != nil && !u.expiresAt.After(now) {
		return ErrRewardExpired
	}

	u.status = RewardActivated
	u.code = code
	u.activatedAt = &now

	return nil
}
