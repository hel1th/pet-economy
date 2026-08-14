package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/shared/auth"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

func TestRoleValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role auth.Role
		want bool
	}{
		{name: "user", role: auth.RoleUser, want: true},
		{name: "moderator", role: auth.RoleModerator, want: true},
		{name: "admin", role: auth.RoleAdmin, want: true},
		{name: "empty", role: auth.Role(""), want: false},
		{name: "unknown", role: auth.Role("root"), want: false},
		{name: "wrong case", role: auth.Role("Admin"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.role.Valid())
		})
	}
}

func TestActorIsZero(t *testing.T) {
	t.Parallel()

	assert.True(t, auth.Actor{}.IsZero())
	assert.True(t, auth.Actor{Role: auth.RoleUser}.IsZero())
	assert.False(t, auth.Actor{ID: uuid.New(), Role: auth.RoleUser}.IsZero())
}

func TestActorHasRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		actor auth.Actor
		roles []auth.Role
		want  bool
	}{
		{
			name:  "matches single role",
			actor: auth.Actor{Role: auth.RoleAdmin},
			roles: []auth.Role{auth.RoleAdmin},
			want:  true,
		},
		{
			name:  "matches one of many",
			actor: auth.Actor{Role: auth.RoleModerator},
			roles: []auth.Role{auth.RoleAdmin, auth.RoleModerator},
			want:  true,
		},
		{
			name:  "no match",
			actor: auth.Actor{Role: auth.RoleUser},
			roles: []auth.Role{auth.RoleAdmin, auth.RoleModerator},
			want:  false,
		},
		{
			name:  "empty role list",
			actor: auth.Actor{Role: auth.RoleAdmin},
			roles: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.actor.HasRole(tt.roles...))
		})
	}
}

func TestWithActorAndActorFrom(t *testing.T) {
	t.Parallel()

	actor := auth.Actor{ID: uuid.New(), Role: auth.RoleModerator}

	got, err := auth.ActorFrom(auth.WithActor(context.Background(), actor))

	require.NoError(t, err)
	assert.Equal(t, actor, got)
}

func TestActorFromMissingOrZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  context.Context
	}{
		{name: "empty context", ctx: context.Background()},
		{name: "zero actor stored", ctx: auth.WithActor(context.Background(), auth.Actor{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := auth.ActorFrom(tt.ctx)

			require.ErrorIs(t, err, domainerr.ErrUnauthorized)
			assert.True(t, got.IsZero())
		})
	}
}
