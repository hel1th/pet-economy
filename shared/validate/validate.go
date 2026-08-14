package validate

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/hel1th/pet-economy/shared/domainerr"
)

type Validator struct {
	v *validator.Validate
}

func New() *Validator {
	return &Validator{v: validator.New(validator.WithRequiredStructEnabled())}
}

func (val *Validator) Struct(dst any) error {
	err := val.v.Struct(dst)
	if err == nil {
		return nil
	}

	var fieldErrors validator.ValidationErrors
	if !errors.As(err, &fieldErrors) || len(fieldErrors) == 0 {
		return domainerr.NewInvalid("body", "invalid payload")
	}

	first := fieldErrors[0]

	return domainerr.NewInvalid(strings.ToLower(first.Field()), messageFor(first))
}

func messageFor(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "field is required"
	case "min":
		return "value is shorter than minimum: " + fe.Param()
	case "max":
		return "value is longer than maximum: " + fe.Param()
	case "gte":
		return "value must be greater than or equal to " + fe.Param()
	case "lte":
		return "value must be less than or equal to " + fe.Param()
	case "email":
		return "invalid email address"
	case "uuid4", "uuid":
		return "invalid identifier"
	case "oneof":
		return "allowed values: " + fe.Param()
	default:
		return "invalid value"
	}
}
