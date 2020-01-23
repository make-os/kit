package util

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	r "math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/robertkrimen/otto"

	"github.com/thoas/go-funk"

	"github.com/fatih/color"

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

	// ZeroString contains "0" value
	ZeroString = String("0")
)

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

// IsDecimal checks whether the string
// can be converted to decimal
func (s String) IsDecimal() bool {
	defer func() {
		_ = recover()
	}()
	s.Decimal()
	return true
}

// ObjectToBytes returns msgpack encoded representation of s.
func ObjectToBytes(s interface{}) []byte {
	var buf bytes.Buffer
	if err := msgpack.NewEncoder(&buf).
		SortMapKeys(true).
		Encode(s); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// BytesToObject decodes bytes produced
// by BytesToObject to the given dest object
func BytesToObject(bs []byte, dest interface{}) error {
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

// LogAlert wraps pp.Println with a read [Alert] prefix.
func LogAlert(format string, args ...interface{}) {
	fmt.Println(color.RedString("[Alert] "+format, args...))
}

// StructToMap returns a map containing fields from the s.
// Map fields are named after their json tags on the struct
func StructToMap(s interface{}) map[string]interface{} {
	_s := structs.New(s)
	_s.TagName = "json"
	return _s.Map()
}

// GetPtrAddr takes a pointer and returns the address
func GetPtrAddr(ptrAddr interface{}) *big.Int {
	ptrAddrInt, ok := new(big.Int).SetString(fmt.Sprintf("%d", &ptrAddr), 10)
	if !ok {
		panic("could not convert pointer address to big.Int")
	}
	return ptrAddrInt
}

// MapDecode decodes a map to a struct.
// It uses mapstructure.Decode internally but
// with 'json' TagName.
func MapDecode(m interface{}, rawVal interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   rawVal,
		TagName:  "json",
	})
	if err != nil {
		return err
	}

	return decoder.Decode(m)
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

// EmptyBlockNonce is a BlockNonce with no values
var EmptyBlockNonce = BlockNonce([8]byte{})

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

// MarshalText encodes n as a hex string with 0x prefix.
func (n BlockNonce) MarshalText() string {
	return ToHex(n[:])
}

// PrintCLIError prints an error message formatted for the command line
func PrintCLIError(msg string, args ...interface{}) {
	fmt.Println(color.RedString("Error:"), fmt.Sprintf(msg, args...))
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

// VMSet sets a value in the vm context only if it
// has been set before.
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

// TouchReader reads one byte from reader into a buffer.
func TouchReader(reader io.Reader) io.Reader {
	bf := bufio.NewReader(reader)
	bf.ReadByte()
	bf.UnreadByte()
	return bf
}

// RemoveFlagVal takes a slice of arguments and remove
// flags matching flagname and their value
func RemoveFlagVal(args []string, flags []string) []string {
	newArgs := []string{}
	curFlag := ""
	curFlagRemoved := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg[:1] == "-" {
			curFlag = strings.Trim(arg[1:], "-")
			curFlag = strings.Split(curFlag, "=")[0]
			if !funk.ContainsString(flags, curFlag) {
				newArgs = append(newArgs, arg)
				curFlagRemoved = false
				continue
			} else {
				curFlagRemoved = true
			}
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

// StructToJSON converts struct to map
func StructToJSON(s interface{}) map[string]interface{} {
	st := structs.New(s)
	st.TagName = "json"
	return st.Map()
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
