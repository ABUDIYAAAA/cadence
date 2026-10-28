package utils

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateStruct validates a struct against validator tags and returns formatted field errors.
func ValidateStruct(s any) map[string]string {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return map[string]string{"error": err.Error()}
	}

	errorsMap := make(map[string]string, len(validationErrors))
	for _, fieldErr := range validationErrors {
		field := fieldErr.Field()
		switch fieldErr.Tag() {
		case "required":
			errorsMap[field] = fmt.Sprintf("%s is required", field)
		case "email":
			errorsMap[field] = fmt.Sprintf("%s must be a valid email address", field)
		case "min":
			errorsMap[field] = fmt.Sprintf("%s must be at least %s characters", field, fieldErr.Param())
		case "max":
			errorsMap[field] = fmt.Sprintf("%s cannot exceed %s characters", field, fieldErr.Param())
		case "eqfield":
			errorsMap[field] = fmt.Sprintf("%s must match %s", field, fieldErr.Param())
		default:
			errorsMap[field] = fmt.Sprintf("%s failed on '%s' rule", field, fieldErr.Tag())
		}
	}

	return errorsMap
}
