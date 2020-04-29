package util

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	r "math/rand"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/btcsuite/btcutil/bech32"
	"github.com/robertkrimen/otto"
	"github.com/thoas/go-funk"

	"github.com/mitchellh/mapstructure"

	"github.com/vmihailenco/msgpack"

	"github.com/fatih/structs"

	"github.com/shopspring/decimal"
)

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits

)

type Map map[string]interface{}

// Big0 represents a zero value big.Int
var Big0 = new(big.Int).SetInt64(0)

func init() {
	r.Seed(time.Now().UnixNano())
}

// String represents a custom string
type String string

// Bytes returns the bytes equivalent of the string
func (s String) Bytes() []byte {
	return []byte(s)
}

// Address converts the String to an Address
func (s String) Address() Address {
	return Address(s)
}

// Equal check whether s and o are the same
func (s String) Equal(o String) bool {
	return s.String() == o.String()
}

func (s String) String() string {
	return string(s)
}

// Empty returns true if the string is empty
func (s String) Empty() bool {
	return len(s) == 0
}

// SS returns a short version of String() with the middle
// characters truncated when length is at least 32
func (s String) SS() string {
	if len(s) >= 32 {
		return fmt.Sprintf("%s...%s", string(s)[0:10], string(s)[len(s)-10:])
	}
	return string(s)
}

// Decimal returns the decimal representation of the string.
// Panics if string failed to be converted to decimal.
func (s String) Decimal() decimal.Decimal {
	return StrToDec(s.String())
}

// Float returns the float equivalent of the numeric value.
// Panics if not convertible to float64
func (s String) Float() float64 {
	f, err := strconv.ParseFloat(string(s), 64)
	if err != nil {
		panic(err)
	}
	return f
}

// IsDecimal checks whether the string can be converted to Decimal
func (s String) IsDecimal() bool {
	return govalidator.IsFloat(string(s))
}

// ToBytes returns msgpack encoded representation of s.
func ToBytes(s interface{}) []byte {
	var buf bytes.Buffer
	if err := msgpack.NewEncoder(&buf).
		SortMapKeys(true).
		Encode(s); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// ToObject decodes bytes produced
// by ToObject to the given dest object
func ToObject(bs []byte, dest interface{}) error {
	return msgpack.NewDecoder(bytes.NewBuffer(bs)).Decode(dest)
}

// RandString is like RandBytes but returns string
func RandString(n int) string {
	return string(RandBytes(n))
}

// RandBytes gets random string of fixed length
func RandBytes(n int) []byte {
	b := make([]byte, n)
	for i, cache, remain := n-1, r.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = r.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return b
}

// NonZeroOrDefIn64 checks if v is 0 so it returns def, otherwise returns v
func NonZeroOrDefIn64(v int64, def int64) int64 {
	if v == 0 {
		return def
	}
	return v
}

// StrToDec converts a numeric string to decimal.
// Panics if val could not be converted to decimal.
func StrToDec(val string) decimal.Decimal {
	d, err := decimal.NewFromString(val)
	if err != nil {
		panic(err)
	}
	return d
}

// IsPathOk checks if a path exist and whether
// there are no permission errors
func IsPathOk(path string) bool {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

// IsFileOk checks if a path exists and it is a file
func IsFileOk(path string) bool {
	s, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return !s.IsDir()
}

// Int64ToHex converts an Int64 value to hex string.
// The resulting hex is prefixed by '0x'
func Int64ToHex(intVal int64) string {
	intValStr := strconv.FormatInt(intVal, 10)
	return "0x" + hex.EncodeToString([]byte(intValStr))
}

// HexToInt64 attempts to convert an hex string to Int64.
// Expects the hex string to begin with '0x'.
func HexToInt64(hexVal string) (int64, error) {
	hexStr, err := hex.DecodeString(hexVal[2:])
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(hexStr), 10, 64)
}

// StrToHex converts a string to hex. T
// The resulting hex is prefixed by '0x'
func StrToHex(str string) string {
	return "0x" + hex.EncodeToString([]byte(str))
}

// HexToStr decodes an hex string to string.
// Expects hexStr to begin with '0x'
func HexToStr(hexStr string) (string, error) {
	bs, err := hex.DecodeString(hexStr[2:])
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// ToHex encodes value to hex with a '0x' prefix
func ToHex(value []byte) string {
	return fmt.Sprintf("0x%s", hex.EncodeToString(value))
}

// FromHex decodes hex value to bytes. If hex value is prefixed
// with '0x' it is trimmed before the decode operation.
func FromHex(hexValue string) ([]byte, error) {
	var _hexValue string
	parts := strings.Split(hexValue, "0x")
	if len(parts) == 1 {
		_hexValue = parts[0]
	} else {
		_hexValue = parts[1]
	}
	return hex.DecodeString(_hexValue)
}

// MustFromHex is like FromHex except it panics if an error occurs
func MustFromHex(hexValue string) []byte {
	v, err := FromHex(hexValue)
	if err != nil {
		panic(err)
	}
	return v
}

// StructToMap converts s to a map.
// If tagName is not provided, 'json' tag is used as a default.
func StructToMap(s interface{}, tagName ...string) map[string]interface{} {
	st := structs.New(s)
	st.TagName = "json"
	if len(tagName) > 0 {
		st.TagName = tagName[0]
	}
	return st.Map()
}

// StructSliceToMapSlice converts a slice of map or struct into a slice of map[string]interface{}
func StructSliceToMapSlice(ss interface{}, tagName ...string) []map[string]interface{} {
	val := reflect.ValueOf(ss)

	if val.Kind() != reflect.Slice {
		panic("arg is not a slice")
	}

	sliceKind := val.Type().Elem().Kind()
	if sliceKind == reflect.Ptr {
		sliceKind = val.Type().Elem().Elem().Kind()
	}
	if sliceKind != reflect.Map && sliceKind != reflect.Struct {
		panic("slice must contain map or struct")
	}

	if val.Len() == 0 {
		return []map[string]interface{}{}
	}

	res := make([]map[string]interface{}, val.Len())
	for i := 0; i < val.Len(); i++ {
		if sliceKind == reflect.Map {
			res[i] = ToMapSI(val.Index(i).Interface(), true)
			continue
		}
		res[i] = StructToMap(val.Index(i).Interface(), tagName...)
	}

	return res
}

// GetPtrAddr takes a pointer and returns the address
func GetPtrAddr(ptrAddr interface{}) *big.Int {
	ptrAddrInt, ok := new(big.Int).SetString(fmt.Sprintf("%d", &ptrAddr), 10)
	if !ok {
		panic("could not convert pointer address to big.Int")
	}
	return ptrAddrInt
}

// DecodeMap decodes a map to a struct.
// It uses mapstructure.Decode internally but
// with 'json' TagName.
func DecodeMap(srcMap interface{}, dest interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   dest,
		TagName:  "json",
	})
	if err != nil {
		return err
	}

	return decoder.Decode(srcMap)
}

// EncodeNumber serializes a number to BigEndian
func EncodeNumber(n uint64) []byte {
	var b = make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

// DecodeNumber deserialize a number from BigEndian
func DecodeNumber(encNum []byte) uint64 {
	return binary.BigEndian.Uint64(encNum)
}

// MayDecodeNumber is like DecodeNumber but returns
// an error instead of panicking
func MayDecodeNumber(encNum []byte) (r uint64, err error) {
	defer func() {
		if rcv, ok := recover().(error); ok {
			err = rcv
		}
	}()
	r = DecodeNumber(encNum)
	return
}

// BlockNonce is a 64-bit hash which proves (combined with the
// mix-hash) that a sufficient amount of computation has been carried
// out on a block.
type BlockNonce [8]byte

// EncodeNonce converts the given integer to a block nonce.
func EncodeNonce(i uint64) BlockNonce {
	var n BlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}

// Uint64 returns the integer value of a block nonce.
func (n BlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}

// IsBoolChanClosed checks whether a boolean channel is closed
func IsBoolChanClosed(c chan bool) bool {
	select {
	case <-c:
		return true
	default:
	}

	return false
}

// IsStructChanClosed checks whether a struct channel is closed
func IsStructChanClosed(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
	}

	return false
}

// IsFuncChanClosed checks whether a function channel is closed
func IsFuncChanClosed(c <-chan func()) bool {
	select {
	case <-c:
		return true
	default:
	}

	return false
}

// VMSet sets a value in the vm context only if it has not been set before.
func VMSet(vm *otto.Otto, name string, value interface{}) interface{} {
	existing, _ := vm.Get(name)
	if !existing.IsUndefined() {
		val, _ := existing.Export()
		return val
	}
	vm.Set(name, value)
	return value
}

// XorBytes xor a and b
func XorBytes(a, b []byte) []byte {
	iA := new(big.Int).SetBytes(a)
	iB := new(big.Int).SetBytes(b)
	return new(big.Int).Xor(iA, iB).Bytes()
}

// RemoveFlag takes a slice of arguments and remove specific flags
func RemoveFlag(args []string, flags []string) []string {
	var newArgs []string
	curFlag := ""
	curFlagRemoved := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg[:1] == "-" {
			curFlag = strings.Split(strings.Trim(arg[1:], "-"), "=")[0]
			if !funk.ContainsString(flags, curFlag) {
				newArgs = append(newArgs, arg)
				curFlagRemoved = false
				continue
			} else {
				if strings.Index(arg, "=") != -1 {
					curFlagRemoved = false
					continue
				}
				curFlagRemoved = true
			}
		} else if arg[:1] != "-" && curFlagRemoved {
			curFlagRemoved = false
			continue
		}
		if !curFlagRemoved {
			newArgs = append(newArgs, arg)
		}
	}
	return newArgs
}

// ParseSimpleArgs passes a programs argument into string
// key and value map.
func ParseSimpleArgs(args []string) (m map[string]string) {
	m = make(map[string]string)
	curFlag := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg[:1] == "-" {
			curFlag = strings.Trim(arg[1:], "-")
			curFlagParts := strings.Split(curFlag, "=")
			curFlag = curFlagParts[0]
			val := ""
			if len(curFlagParts) >= 2 {
				val = curFlagParts[1]
				m[curFlag] = val
				curFlag = ""
			}
			continue
		}
		if curFlag != "" {
			m[curFlag] = arg
			curFlag = ""
			continue
		}
	}
	return
}

// Interrupt is used to signal program interruption
type Interrupt chan struct{}

// IsClosed checks if the channel is closed
func (i *Interrupt) IsClosed() bool {
	return IsStructChanClosed(*i)
}

// Close closes the channel only when it is not closed
func (i *Interrupt) Close() {
	if !i.IsClosed() {
		close(*i)
	}
}

// Wait blocks the calling goroutine till the channel is closed
func (i *Interrupt) Wait() {
	<-*i
}

// ParseExtArgs parses an extension arguments map.
// It takes a map of the form:
// 'extName.arg = value' and returns 'extName={arg=value...arg2=value2}'
func ParseExtArgs(extArgs map[string]string) (extsArgs map[string]map[string]string, common map[string]string) {
	extsArgs = make(map[string]map[string]string)
	common = make(map[string]string)
	for k, v := range extArgs {
		if strings.Index(k, ".") == -1 {
			common[k] = v
			continue
		}
		kPart := strings.Split(k, ".")
		argM, ok := extsArgs[kPart[0]]
		if !ok {
			extsArgs[kPart[0]] = map[string]string{}
			argM = map[string]string{}
		}
		argM[kPart[1]] = v
		extsArgs[kPart[0]] = argM
	}
	return
}

// CopyMap copies src map to dst
func CopyMap(src, dst map[string]interface{}) {
	for k, v := range src {
		dst[k] = v
	}
}

// CloneMap copies src map to a new map
func CloneMap(src map[string]interface{}) (dst map[string]interface{}) {
	dst = make(map[string]interface{})
	for k, v := range src {
		dst[k] = v
	}
	return
}

// SplitNamespaceDomain splits a full namespace address into namespace and domain
func SplitNamespaceDomain(name string) (ns string, domain string, err error) {
	parts := strings.Split(name, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid address format")
	}
	return parts[0], parts[1], nil
}

// IsValidAddr checks whether an address is valid
func IsValidAddr(addr string) error {
	if addr == "" {
		return fmt.Errorf("empty address")
	}

	_, _, err := bech32.Decode(addr)
	if err != nil {
		return err
	}

	return nil
}

// WriteJSON encodes respObj to json and writes it to w
func WriteJSON(w http.ResponseWriter, statuscode int, respObj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statuscode)
	json.NewEncoder(w).Encode(respObj)
}

// RESTApiErrorMsg returns a message suitable for reporting REST API errors
func RESTApiErrorMsg(msg, field string, code string) map[string]interface{} {
	obj := make(map[string]interface{})
	obj["msg"] = msg
	if field != "" {
		obj["field"] = field
	}
	if code != "" {
		obj["code"] = code
	}
	return map[string]interface{}{
		"error": obj,
	}
}

// GetIndexFromUInt64Slice gets an index from a uint64 variadic param.
// Returns 0 if opts is empty
func GetIndexFromUInt64Slice(index int, opts ...uint64) uint64 {
	if len(opts) == 0 {
		return 0
	}
	return opts[index]
}

// ToMapSI converts a map to map[string]interface{}.
// If structToMap is true, struct element is converted to map.
// Panics if m is not a map with string key.
// Returns m if m is already a map[string]interface{}.
func ToMapSI(m interface{}, structToMap ...bool) map[string]interface{} {
	v := reflect.ValueOf(m)
	if v.Kind() != reflect.Map || v.Type().Key().Kind() != reflect.String {
		panic("not a map with string key")
	}

	if v.Type().Elem().Kind() == reflect.Interface {
		return m.(map[string]interface{})
	}

	res := make(map[string]interface{})
	for _, k := range v.MapKeys() {
		mapVal := v.MapIndex(k)
		if len(structToMap) > 0 && structToMap[0] &&
			(mapVal.Kind() == reflect.Struct ||
				(mapVal.Kind() == reflect.Ptr && mapVal.Elem().Kind() == reflect.Struct)) {
			res[k.String()] = StructToMap(mapVal.Interface())
			continue
		}
		res[k.String()] = v.MapIndex(k).Interface()
	}

	return res
}

// IsZeroString returns true if str is empty or equal "0"
func IsZeroString(str string) bool {
	return str == "" || str == "0"
}

// IsValidIdentifierName
func IsValidIdentifierName(name string) error {
	if !govalidator.Matches(name, "^[a-zA-Z0-9_-]+$") {
		return fmt.Errorf("invalid characters in name. Only alphanumeric, _ and - characters are allowed")
	} else if len(name) > 128 {
		return fmt.Errorf("name is too long. Maximum character length is 128")
	} else if len(name) <= 2 {
		return fmt.Errorf("name is too short. Must be at least 3 characters long")
	}
	return nil
}

// ReadFromEditor reads input from the specified editor program
func ReadFromEditor(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error) {
	file, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		return "", nil
	}
	defer os.Remove(file.Name())

	args := strings.Split(editor, " ")
	cmd := exec.Command(args[0], append(args[1:], file.Name())...)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	cmd.Stdin = stdIn
	if err := cmd.Run(); err != nil {
		return "", err
	}

	bz, err := ioutil.ReadFile(file.Name())
	if err != nil {
		return "", err
	}

	return string(bz), nil
}
