package identifier

import (
	"fmt"

	"github.com/asaskevich/govalidator"
)

// IsValidResourceName checks whether a user-defined name is valid as a resource name
func IsValidResourceName(name string) error {
	if len(name) <= 2 {
		return fmt.Errorf("name is too short. Must be at least 3 characters long")
	}
	if !govalidator.Matches(name, "^[a-z0-9][a-zA-Z0-9_-]+$") {
		if name != "" && (name[0] == '_' || name[0] == '-') {
			return fmt.Errorf("invalid identifier; identifier cannot start with _ or - character")
		}
		return fmt.Errorf("invalid identifier; only alphanumeric, _, and - characters are allowed")
	}
	if len(name) > 128 {
		return fmt.Errorf("name is too long. Maximum character length is 128")
	}
	return nil
}

// IsValidResourceNameNoMinLen is like IsValidResourceName but minimum length is not enforced.
func IsValidResourceNameNoMinLen(name string) error {
	if !govalidator.Matches(name, "^[a-z0-9]([a-zA-Z0-9_-]+)?$") {
		if name != "" && (name[0] == '_' || name[0] == '-') {
			return fmt.Errorf("invalid identifier; identifier cannot start with _ or - character")
		}
		return fmt.Errorf("invalid identifier; only alphanumeric, _, and - characters are allowed")
	}
	if len(name) > 128 {
		return fmt.Errorf("name is too long. Maximum character length is 128")
	}
	return nil
}
