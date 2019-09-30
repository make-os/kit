package types

import (
	"fmt"
)

// FieldError is used to describe an error concerning an objects field/property
func FieldError(field, err string) error {
	return fmt.Errorf(fmt.Sprintf("field:%s, error:%s", field, err))
}

// FieldErrorWithIndex is used to describe an error concerning an field/property
// of an object contained in list (array or slice).
// If index is -1, it will revert to FieldError
func FieldErrorWithIndex(index int, field, err string) error {
	if index == -1 {
		return FieldError(field, err)
	}
	var fieldArg = "field:%s, "
	if field == "" {
		fieldArg = "%s"
	}
	return fmt.Errorf(fmt.Sprintf("index:%d, "+fieldArg+"error:%s", index, field, err))
}
