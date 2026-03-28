package errors

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// FormatValidationErrors converts validator.ValidationErrors into a
// human-readable slice of strings.
func FormatValidationErrors(err error) []string {
	ve, ok := err.(validator.ValidationErrors)
	if !ok {
		return []string{err.Error()}
	}
	out := make([]string, 0, len(ve))
	for _, fe := range ve {
		out = append(out, fmt.Sprintf("field '%s' failed on '%s' validation",
			strings.ToLower(fe.Field()), fe.Tag()))
	}
	return out
}
