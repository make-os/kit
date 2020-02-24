package jsonrpc

import (
	"strconv"

	"github.com/asaskevich/govalidator"
	"github.com/stretchr/objx"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util"
)

// GetStringFromObjxMap gets a field from the given objx.Map ensuring its type is a 'string'.
//
// o: The objx map.
// field: The target field to get.
// required: Whether to return error if field does not exist in the object map.
func GetStringFromObjxMap(o objx.Map, field string, required bool) (string, *Response) {

	if required && !o.Has(field) {
		msg := util.FieldError(field, field+" is required").Error()
		return "", Error(types.RPCErrCodeInvalidParamValue, msg, nil)
	} else if !o.Has(field) {
		return "", nil
	}

	vField := o.Get(field)
	if !vField.IsStr() {
		msg := util.WrongFieldValueMsg(field, "string", vField.Inter()).Error()
		return "", Error(types.RPCErrCodeInvalidParamValue, msg, nil)
	}

	return vField.Str(), nil
}

// GetStringToUint64FromObjxMap gets a field from the given objx.Map ensuring
// its type is a 'string' that is convertible to Uint64
//
// o: The objx map.
// field: The target field to get.
// required: Whether to return error if field does not exist in the object map.
func GetStringToUint64FromObjxMap(o objx.Map, field string, required bool) (uint64, *Response) {

	if required && !o.Has(field) {
		msg := util.FieldError(field, field+" is required").Error()
		return 0, Error(types.RPCErrCodeInvalidParamValue, msg, nil)
	} else if !o.Has(field) {
		return 0, nil
	}

	vField := o.Get(field)
	if !vField.IsStr() {
		msg := util.WrongFieldValueMsg(field, "string", vField.Inter()).Error()
		return 0, Error(types.RPCErrCodeInvalidParamValue, msg, nil)
	}
	if vField.Str() != "" {
		if govalidator.IsNumeric(vField.Str()) {
			msg := util.FieldError(field, "numeric value required").Error()
			return 0, Error(types.RPCErrCodeInvalidParamValue, msg, nil)
		}
	}
	fieldValue, err := strconv.ParseUint(vField.Str(), 10, 64)
	if err != nil {
		return 0, Error(types.RPCErrCodeInvalidParamValue,
			"failed to parse to unsigned integer", nil)
	}

	return fieldValue, nil
}
