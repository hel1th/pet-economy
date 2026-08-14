package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/shared/domainerr"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleModerator Role = "moderator"
	RoleAdmin     Role = "admin"
)

func (r Role) Valid() bool {
	switch r {
	case RoleUser, RoleModerator, RoleAdmin:
		return true
	default:
		return false
	}
}

type Actor struct {
	ID   uuid.UUID
	Role Role
}

func (a Actor) IsZero() bool {
	return a.ID == uuid.Nil
}

func (a Actor) HasRole(roles ...Role) bool {
	for _, role := range roles {
		if a.Role == role {
			return true
		}
	}

	return false
}

type actorKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

func ActorFrom(ctx context.Context) (Actor, error) {
	actor, ok := ctx.Value(actorKey{}).(Actor)
	if !ok || actor.IsZero() {
		return Actor{}, domainerr.ErrUnauthorized
	}

	return actor, nil
}
