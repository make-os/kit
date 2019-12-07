package dht

import "errors"

type validator struct {
}

// Validate conforms to the Validator interface.
func (v validator) Validate(key string, value []byte) error {
	return nil
}

// Select conforms to the Validator interface.
func (v validator) Select(key string, values [][]byte) (int, error) {
	if len(values) == 0 {
		return 0, errors.New("can't select from no values")
	}
	return 0, nil
}
