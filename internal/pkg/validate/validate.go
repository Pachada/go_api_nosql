package validate

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// v is the package-level singleton validator. It is initialised once at
// package load time. Any custom type registrations must be made during init()
// before the first call to Struct.
var v = validator.New()

// Struct validates the given struct using its validate tags.
// Returns a human-readable error string or nil.
func Struct(s interface{}) error {
	if err := v.Struct(s); err != nil {
		ve, ok := err.(validator.ValidationErrors)
		if !ok {
			return err
		}
		var msgs []string
		for _, fe := range ve {
			msgs = append(msgs, fmt.Sprintf("field '%s' failed '%s'", fe.Field(), fe.Tag()))
		}
		return fmt.Errorf("%s", strings.Join(msgs, "; "))
	}
	return nil
}
