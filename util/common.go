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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/mitchellh/mapstructure"
	"github.com/robertkrimen/otto"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"
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

// ToBytes returns msgpack encoded representation of s.
func ToBytes(s interface{}) []byte {
	var buf bytes.Buffer
	if err := msgpack.NewEncoder(&buf).
		SortMapKeys(true).
		UseCompactEncoding(true).
		Encode(s); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// ToObject decodes bytes produced by ToBytes to the given dest object
func ToObject(bs []byte, dest interface{}) error {
	return msgpack.NewDecoder(bytes.NewBuffer(bs)).Decode(dest)
}

// MustToJSON converts the give obj to valid JSON.
// Panics if unsuccessful.
func MustToJSON(obj interface{}) string {
	res, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return string(res)
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

// ToHexBytes encodes source to hex bytes
func ToHexBytes(src []byte) []byte {
	var dst = make([]byte, hex.EncodedLen(len(src)))
	hex.Encode(dst, src)
	return dst
}

// FromHexBytes decodes hex source to bytes
func FromHexBytes(src []byte) []byte {
	var dst = make([]byte, hex.DecodedLen(len(src)))
	hex.Decode(dst, src)
	return dst
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

// DecodeWithJSON is like DecodeMap but it marshals src and Unmarshal to dest using encode/json.
func DecodeWithJSON(src interface{}, dest interface{}) error {
	bz, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(bz, dest)
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
func RemoveFlag(args []string, flags ...string) []string {
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

// EditorReaderFunc describes a function that reads input from an editor program
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

// ParseVerbs parses a template containing verbs prefixed with %.
func ParseVerbs(tmp string, verbsValue map[string]interface{}) string {

	// Get the keys and sort them in descending order
	var verbs = funk.Keys(verbsValue).([]string)
	sort.Strings(verbs)
	verbs = funk.ReverseStrings(verbs)

	for _, k := range verbs {
		tmp = strings.Replace(tmp, "%"+k, fmt.Sprintf("%v", verbsValue[k]), -1)
	}

	return tmp
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

// ParseGitVersion extracts semver version from `git version` output
func ParseGitVersion(version string) (ver string) {
	verSplit := strings.SplitN(version, ".", 4)
	if len(verSplit) < 3 {
		return strings.Join(verSplit, ".")
	}
	return strings.Join(verSplit[:3], ".")
}

// IsGitInstalled checks whether git executable exist in the given path.
// Returns true and git version if installed or false.
func IsGitInstalled(path string) (bool, string) {
	cmd := exec.Command(path, "version")
	out, err := cmd.Output()
	if err != nil {
		return false, ""
	}
	return true, ParseGitVersion(strings.Fields(string(out))[2])
}

// Notify displays a desktop notification
func Notify(val ...interface{}) {
	beeep.Alert("", fmt.Sprint(val...), "")
}

// ParseLogLevel parse value from --loglevel flag
func ParseLogLevel(val string) (res map[string]logrus.Level) {
	res = map[string]logrus.Level{}
	logLev := strings.TrimRight(strings.TrimLeft(val, "["), "]")
	for _, str := range strings.Split(logLev, ",") {
		str = strings.TrimSpace(str)
		parts := strings.Split(str, "=")
		if len(parts) != 2 {
			continue
		}
		module := strings.TrimSpace(parts[0])
		lvl := strings.TrimSpace(parts[1])
		lvlCast, err := cast.ToUint32E(lvl)
		if err == nil {
			res[module] = logrus.Level(lvlCast)
		}
	}
	return
}

// ToByteSlice converts a uint slice to byte slice
func ToByteSlice(v []int) []byte {
	bz := make([]byte, len(v))
	for i, vv := range v {
		bz[i] = byte(vv)
	}
	return bz
}

// FatalOnError logs a message of Fatal level if err is not nil
func FatalOnError(err error) {
	if err != nil {
		logrus.Fatal(err)
	}
}
