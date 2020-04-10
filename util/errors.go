package util

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// BadFieldError implements error, for describing an invalid input.
// It outputs the example format: `field:id, msg:some error message about id, index:1`
type BadFieldError struct {
	Field string
	Msg   string
	Index int
}

func (b *BadFieldError) Is(target error) bool {
	_, ok := target.(*BadFieldError)
	return ok
}

func (b *BadFieldError) Error() string {
	return fieldErrorWithIndex(b.Index, b.Field, b.Msg).Error()
}

// FieldErrorWithIndex creates an instance of BadFieldError with an index
func FieldErrorWithIndex(index int, field string, msg string) error {

	// Prevent the occurrence of ':' after a ',' as it will cause the
	// output to be incorrectly parsed back to a BadFieldError.
	if commaIdx := strings.Index(msg, ","); commaIdx > -1 && commaIdx < strings.Index(msg, ",") {
		panic("FieldErrorWithIndex: message cannot include `:` character after a ',' character")
	}

	return &BadFieldError{Field: field, Msg: msg, Index: index}
}

// FieldError creates an instance of BadFieldError without an index
func FieldError(field string, msg string) error {
	return &BadFieldError{Field: field, Msg: msg, Index: -1}
}

// CallOnNilErr calls f if err is nil
func CallOnNilErr(err error, f func() error) error {
	if err == nil {
		err = f()
	}
	return err
}

// FieldError is used to describe an error concerning an objects field/property
func fieldError(field, err string) error {
	return fmt.Errorf(fmt.Sprintf("field:%s, msg:%s", field, err))
}

// FieldErrorWithIndex is used to describe an error concerning an field/property
// of an object contained in list (array or slice).
// If index is -1, it will revert to FieldError
func fieldErrorWithIndex(index int, field, err string) error {
	if index == -1 {
		return fieldError(field, err)
	}
	var fieldArg = "field:%s, "
	if field == "" {
		fieldArg = "%s"
	}
	return fmt.Errorf(fmt.Sprintf("index:%d, "+fieldArg+"msg:%s", index, field, err))
}

// WrongFieldValueMsg generates a message to indicate an unexpected field value type
func WrongFieldValueMsg(expectedType string, actual interface{}) string {
	return fmt.Sprintf("wrong value type, want '%s', got %T",
		expectedType, reflect.TypeOf(actual).String())
}

// StatusError describes a module error with information that allows
// the error to be interpreted correctly by other dependent packages
// such as rest and json-rpc services.
type StatusError struct {
	Code     string
	HttpCode int
	Msg      string
	Field    string
}

// NewStatusError creates StatusError
// It outputs the example format: `msg:'some error message', httpCode:'400', code:'mempool_add_fail, field:'id'`
func NewStatusError(httpCode int, code, field, msg string) *StatusError {
	return &StatusError{Code: code, HttpCode: httpCode, Msg: msg, Field: field}
}

func (s *StatusError) Error() string {
	var msgParts []string
	if s.Field != "" {
		msgParts = append(msgParts, fmt.Sprintf("field:'%s'", s.Field))
	}
	if s.Msg != "" {
		msgParts = append(msgParts, fmt.Sprintf("msg:'%s'", s.Msg))
	}
	if s.HttpCode != 0 {
		msgParts = append(msgParts, fmt.Sprintf("httpCode:'%d'", s.HttpCode))
	}
	if s.Code != "" {
		msgParts = append(msgParts, fmt.Sprintf("code:'%s'", s.Code))
	}
	return strings.Join(msgParts, ", ")
}

func (s *StatusError) Is(target error) bool {
	_, ok := target.(*StatusError)
	return ok
}

// StatusErrorFromStr attempts to convert a string to a StatusError. It expects the
// string to match the StatusError#Error output.
// Never returns an error even on failure.
func StatusErrorFromStr(str string) *StatusError {
	var msgRe = regexp.MustCompile(`(?m)msg:'(.*?)'(,|$|\s)`)
	var httpCodeRe = regexp.MustCompile(`(?m)httpCode:'(.*?)'`)
	var codeRe = regexp.MustCompile(`(?m)code:'(.*?)'`)
	var fieldRe = regexp.MustCompile(`(?m)field:'(.*?)'`)

	err := &StatusError{}
	if res := msgRe.FindStringSubmatch(str); res != nil {
		err.Msg = res[1]
	}

	if res := httpCodeRe.FindStringSubmatch(str); res != nil {
		httpCode, _ := strconv.Atoi(res[1])
		err.HttpCode = httpCode
	}

	if res := codeRe.FindStringSubmatch(str); res != nil {
		err.Code = res[1]
	}

	if res := fieldRe.FindStringSubmatch(str); res != nil {
		err.Field = res[1]
	}

	return err
}

// getKeyFromFieldErrOutput lets you extract the value of a key in a BadFieldError output
func getKeyFromFieldErrOutput(fieldErr, key string) string {
	target := key + ":"
	var buf2 []byte
	var buf []byte
	startIndex, endIndex := -1, -1

	for i := 0; i < len(fieldErr); i++ {
		buf = append(buf, fieldErr[i])
		idx := len(buf) - len(target)
		if idx < 0 {
			continue
		}
		if string(buf[idx:]) == target {
			startIndex = i
			continue
		}
		if startIndex > -1 {
			buf2 = append(buf2, fieldErr[i])

			if string(fieldErr[i]) == "," {
				endIndex = i
			}

			if endIndex > -1 && string(fieldErr[i]) == ":" {
				break
			}
		}
	}

	if endIndex > -1 {
		buf2 = buf2[:endIndex-startIndex-1]
	}

	return string(buf2)
}

// BadFieldErrorFromStr attempts to convert a string to a BadFieldError. It expects the
// string to match the BadFieldError#Error output.
// Never returns an error even on failure.
func BadFieldErrorFromStr(str string) *BadFieldError {
	fe := &BadFieldError{
		Field: getKeyFromFieldErrOutput(str, "field"),
		Msg:   getKeyFromFieldErrOutput(str, "msg"),
	}
	index := getKeyFromFieldErrOutput(str, "index")
	if index != "" {
		indexInt, _ := strconv.Atoi(index)
		fe.Index = indexInt
	}
	return fe
}
