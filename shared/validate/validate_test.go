package validate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/shared/domainerr"
	"github.com/hel1th/pet-economy/shared/validate"
)

type form struct {
	Email  string `validate:"required,email"`
	Title  string `validate:"required,min=3,max=10"`
	Amount int    `validate:"gte=0,lte=100"`
	Status string `validate:"omitempty,oneof=draft published"`
	ID     string `validate:"omitempty,uuid4"`
	Color  string `validate:"omitempty,hexcolor"`
}

func validForm() form {
	return form{Email: "user@example.com", Title: "chair", Amount: 10}
}

func TestValidatorAcceptsValidStruct(t *testing.T) {
	t.Parallel()

	require.NoError(t, validate.New().Struct(validForm()))
}

func TestValidatorMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*form)
		wantField string
		wantMsg   string
	}{
		{
			name:      "required",
			mutate:    func(f *form) { f.Title = "" },
			wantField: "title",
			wantMsg:   "field is required",
		},
		{
			name:      "min",
			mutate:    func(f *form) { f.Title = "ab" },
			wantField: "title",
			wantMsg:   "value is shorter than minimum: 3",
		},
		{
			name:      "max",
			mutate:    func(f *form) { f.Title = "way too long title" },
			wantField: "title",
			wantMsg:   "value is longer than maximum: 10",
		},
		{
			name:      "gte",
			mutate:    func(f *form) { f.Amount = -1 },
			wantField: "amount",
			wantMsg:   "value must be greater than or equal to 0",
		},
		{
			name:      "lte",
			mutate:    func(f *form) { f.Amount = 101 },
			wantField: "amount",
			wantMsg:   "value must be less than or equal to 100",
		},
		{
			name:      "email",
			mutate:    func(f *form) { f.Email = "not-an-email" },
			wantField: "email",
			wantMsg:   "invalid email address",
		},
		{
			name:      "uuid4",
			mutate:    func(f *form) { f.ID = "1234" },
			wantField: "id",
			wantMsg:   "invalid identifier",
		},
		{
			name:      "oneof",
			mutate:    func(f *form) { f.Status = "sold" },
			wantField: "status",
			wantMsg:   "allowed values: draft published",
		},
		{
			name:      "unknown tag falls back",
			mutate:    func(f *form) { f.Color = "not-a-color" },
			wantField: "color",
			wantMsg:   "invalid value",
		},
	}

	validator := validate.New()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := validForm()
			tc.mutate(&value)

			err := validator.Struct(value)

			var invalid *domainerr.InvalidError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, tc.wantField, invalid.Field)
			assert.Equal(t, tc.wantMsg, invalid.Message)
		})
	}
}

func TestValidatorRejectsNonStruct(t *testing.T) {
	t.Parallel()

	err := validate.New().Struct("not a struct")

	var invalid *domainerr.InvalidError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "body", invalid.Field)
}
