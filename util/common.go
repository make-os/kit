package util

import (
	"bufio"
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
	"github.com/cbroglie/mustache"
	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/mitchellh/mapstructure"
	"github.com/robertkrimen/otto"
	"github.com/thoas/go-funk"

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

// IsZero returns true if str is empty or equal "0"
func (s String) IsZero() bool {
	return IsZeroString(string(s))
}

// IsNumeric checks whether s is numeric
func (s String) IsNumeric() bool {
	return govalidator.IsFloat(s.String())
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

// ToObject decodes bytes produced by ToBytes to the given dest object
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

// NonZeroOrDefIn64 returns v if non-zero, otherwise it returns def
func NonZeroOrDefIn64(v int64, def int64) int64 {
	if v == 0 {
		return def
	}
	return v
}

// NonZeroOrDefString returns v if non-zero, otherwise it returns def
func NonZeroOrDefString(v string, def string) string {
	if v == "" {
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

// ToHex encodes value to hex with a '0x' prefix only if noPrefix is unset
func ToHex(value []byte, noPrefix ...bool) string {
	if len(noPrefix) > 0 && noPrefix[0] {
		return hex.EncodeToString(value)
	}
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

// StructSliceToMap converts struct slice s to a map slice.
// If tagName is not provided, 'json' tag is used as a default.
func StructSliceToMap(s interface{}, tagName ...string) []Map {
	val := reflect.ValueOf(s)
	switch val.Kind() {
	case reflect.Slice:
		var res = []Map{}
		for i := 0; i < val.Len(); i++ {
			res = append(res, ToMap(val.Index(i).Interface(), tagName...))
		}
		return res
	default:
		panic("slice of struct was expected")
	}
}

// ToBasicMap converts s to a map stripping out custom types.
// Panics if s cannot be (un)marshalled by encoding/json package.
func ToBasicMap(s interface{}) (m map[string]interface{}) {
	bz, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(bz, &m); err != nil {
		panic(err)
	}
	return
}

// ToBasicMap converts s to a map.
// If tagName is not provided, 'json' tag is used as a default.
func ToMap(s interface{}, tagName ...string) map[string]interface{} {
	st := structs.New(s)
	st.TagName = "json"
	if len(tagName) > 0 {
		st.TagName = tagName[0]
	}
	return st.Map()
}

// GetPtrAddr takes a pointer and returns the address
func GetPtrAddr(ptrAddr interface{}) *big.Int {
	ptrAddrInt, ok := new(big.Int).SetString(fmt.Sprintf("%d", &ptrAddr), 10)
	if !ok {
		panic("could not convert pointer address to big.Int")
	}
	return ptrAddrInt
}

// DecodeMapStrict decodes a map to a struct with strict type check.
// Default tagname is 'json'
func DecodeMapStrict(srcMap interface{}, dest interface{}, tagName ...string) error {
	tn := "json"
	if len(tagName) > 0 {
		tn = tagName[0]
	}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   dest,
		TagName:  tn,
	})
	if err != nil {
		return err
	}

	return decoder.Decode(srcMap)
}

// DecodeMap decodes a map to a struct with weak conversion.
// Default tagname is 'json'
func DecodeMap(srcMap interface{}, dest interface{}, tagName ...string) error {
	tn := "json"
	if len(tagName) > 0 {
		tn = tagName[0]
	}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           dest,
		TagName:          tn,
		WeaklyTypedInput: true,
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

// Interrupt is used to signal program interruption
type Interrupt chan struct{}

// CloseIsTrue checks if the channel is closed
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

// ToStringMapInter converts a map to map[string]interface{}.
// If structToMap is true, struct element is converted to map.
// Panics if m is not a map with string key.
// Returns m if m is already a map[string]interface{}.
func ToStringMapInter(m interface{}, structToMap ...bool) map[string]interface{} {
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
			res[k.String()] = ToMap(mapVal.Interface())
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

// IsValidName checks whether a user-defined identifier/name is valid
func IsValidName(name string) error {
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

// IsValidNameNoLen checks whether a user-defined identifier/name is valid but it does not enforce a length requirement
func IsValidNameNoLen(name string) error {
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

// EditorReaderFunc describes a function that collects input from an editor program
type EditorReaderFunc func(editor string, stdIn io.Reader, stdOut, stdErr io.Writer) (string, error)

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

// MustacheParserOpt are options for MustacheParseString
type MustacheParserOpt struct {
	ForceRaw bool
	StartTag string
	EndTag   string
}

// MustacheParseString passes a given string format.
func MustacheParseString(format string, ctx map[string]interface{}, opt MustacheParserOpt) (str string, err error) {
	defer func() {
		if rcv, ok := recover().(error); ok {
			err = rcv
		}
	}()
	tpl, err := mustache.ParseStringPartialsRawWithDelims(format, nil,
		opt.StartTag, opt.EndTag, opt.ForceRaw)
	if err != nil {
		return "", err
	}
	return tpl.Render(ctx)
}

// IsString checks whether the interface is a string
func IsString(v interface{}) bool {
	return reflect.TypeOf(v).Kind() == reflect.String
}

// RemoveFromStringSlice removes str from the given string slice
func RemoveFromStringSlice(slice []string, str string) []string {
	return funk.FilterString(slice, func(s string) bool { return s != str })
}

// ParseFrontMatterContent parses a reader containing front matter + content.
func ParseContentFrontMatter(rdr io.Reader) (pageparser.ContentFrontMatter, error) {
	buf := bufio.NewReader(rdr)
	d, err := buf.Peek(4)
	if err != nil && err != io.EOF {
		return pageparser.ContentFrontMatter{}, err
	}

	// If first 4 characters does not begin with `---\n`, then the reader
	// contains only content (no front matter), just return ContentFrontMatter with content set
	if string(d) != "---\n" || err == io.EOF {
		bz, _ := ioutil.ReadAll(buf)
		return pageparser.ContentFrontMatter{Content: bz}, nil
	}

	return pageparser.ParseFrontMatterAndContent(buf)
}

// IsMapOrStruct checks whether o is a map or a struct (pointer to struct)
func IsMapOrStruct(o interface{}) bool {
	typ := reflect.TypeOf(o)
	if typ.Kind() == reflect.Struct || typ.Kind() == reflect.Map {
		return true
	}
	if typ.Kind() == reflect.Ptr && typ.Elem().Kind() == reflect.Struct {
		return true
	}
	return false
}

// ParseUint is like strconv.ParseUint but returns util.UIInt64
func ParseUint(s string, base int, bitSize int) (UInt64, error) {
	v, err := strconv.ParseUint(s, base, bitSize)
	if err != nil {
		return 0, err
	}
	return UInt64(v), err
}
