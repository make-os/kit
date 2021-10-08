package errors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/make-os/kit/util"
	"github.com/spf13/cast"
)

// BadFieldError implements error. It describes an error relating to an object and/or field.
type BadFieldError struct {
	Field string `json:"field,omitempty"`
	Msg   string `json:"msg"`
	Index *int   `json:"index,omitempty"`
	Data  interface{}
}

func (b *BadFieldError) Is(target error) bool {
	_, ok := target.(*BadFieldError)
	return ok
}

func (b *BadFieldError) Error() string {
	return fieldErrorWithIndex(b.Index, b.Field, b.Msg).Error()
}

// FieldErrorWithIndex creates an instance of BadFieldError with an index
func FieldErrorWithIndex(index int, field string, msg string, data ...interface{}) error {
	e := &BadFieldError{Field: field, Msg: msg}
	if index > -1 {
		e.Index = &index
	}
	if len(data) > 0 {
		e.Data = data[0]
	}
	return e
}

// FieldError creates an instance of BadFieldError without an index
func FieldError(field string, msg string) error {
	return &BadFieldError{Field: field, Msg: msg, Index: nil}
}

// CallIfNil calls f if err is nil
func CallIfNil(err error, f func() error) error {
	if err == nil {
		err = f()
	}
	return err
}

// FieldError is used to describe an error concerning an objects field/property
func fieldError(field, err string) error {
	return fmt.Errorf(mapToJSONWithNoBrackets(map[string]string{
		"field": field,
		"msg":   err,
	}))
}

func mapToJSONWithNoBrackets(m map[string]string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.Encode(m)
	data := strings.TrimSpace(buf.String())
	return strings.TrimSpace(strings.Trim(strings.Trim(data, "{"), "}"))
}

// fieldErrorWithIndex is used to describe an error concerning a field/property
// of an object contained in list (array or slice).
// If index is -1, it will revert to FieldError
func fieldErrorWithIndex(index *int, field, err string) error {
	if index == nil || *index <= -1 {
		return fieldError(field, err)
	}
	return fmt.Errorf(mapToJSONWithNoBrackets(map[string]string{
		"index": cast.ToString(*index),
		"field": field,
		"msg":   err,
	}))
}

// ReqError describes an error consumable by http services.
type ReqError struct {
	Code     string
	HttpCode int
	Msg      string
	Field    string
}

// ReqErr creates ReqError
func ReqErr(httpCode int, code, field, msg string) *ReqError {
	return &ReqError{Code: code, HttpCode: httpCode, Msg: msg, Field: field}
}

// IsSet returns true if code, http code and msg fields are set
func (s *ReqError) IsSet() bool {
	return s.Code != "" && s.HttpCode != 0 && s.Msg != ""
}

func (s ReqError) String() string {
	return s.Error()
}

func (s *ReqError) Error() string {
	return mapToJSONWithNoBrackets(map[string]string{
		"field":    s.Field,
		"msg":      s.Msg,
		"httpCode": cast.ToString(s.HttpCode),
		"code":     s.Code,
	})
}

// ReqErrorFromStr attempts to convert a string to a ReqError. It expects the
// string to match the ReqError.Error output.
// Never returns an error even on failure.
func ReqErrorFromStr(str string) *ReqError {
	jsonStr := `{` + str + `}`
	m := make(map[string]string)
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return &ReqError{Msg: str}
	}
	var re ReqError
	util.DecodeMap(m, &re)
	return &re
}

// BadFieldErrorFromStr attempts to convert a string to a BadFieldError.
// It expects the string to match the BadFieldError.Error output.
func BadFieldErrorFromStr(str string) *BadFieldError {
	jsonStr := `{` + str + `}`
	m := make(map[string]string)
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return &BadFieldError{Msg: str}
	}
	var fe BadFieldError
	util.DecodeMap(m, &fe)
	return &fe
}
