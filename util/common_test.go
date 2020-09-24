package util

import (
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/robertkrimen/otto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type CustomInt int

func (a CustomInt) MarshalJSON() ([]byte, error) {
	var result string
	if a == 0 {
		result = `0`
	} else {
		result = fmt.Sprintf("%d", a)
	}
	return []byte(result), nil
}

var _ = Describe("Common", func() {

	Describe(".ObjectsToBytes", func() {
		It("should return expected bytes", func() {
			s := struct{ Name string }{Name: "ben"}
			expected := []uint8{
				0x81, 0xa4, 0x4e, 0x61, 0x6d, 0x65, 0xa3, 0x62, 0x65, 0x6e,
			}
			bs := ToBytes(s)
			Expect(bs).To(Equal(expected))
		})
	})

	Describe(".ParseVerbs", func() {
		It("should test cases correctly", func() {
			out := ParseVerbs("We have %d by %d planks. My friend %T will sell them", map[string]interface{}{"d": 2, "T": "Odion"})
			Expect(out).To(Equal("We have 2 by 2 planks. My friend Odion will sell them"))
			out = ParseVerbs("%th by %th, %t", map[string]interface{}{"th": 2, "t": "Odion"})
			Expect(out).To(Equal("2 by 2, Odion"))
			out = ParseVerbs("%t by %t, %th", map[string]interface{}{"th": 2, "t": "Odion"})
			Expect(out).To(Equal("Odion by Odion, 2"))
		})
	})

	Describe(".ToObject", func() {

		var bs []byte
		var m = map[string]interface{}{"stuff": int8(10)}

		BeforeEach(func() {
			bs = ToBytes(m)
			Expect(bs).ToNot(BeEmpty())
		})

		It("should decode to expected value", func() {
			var actual map[string]interface{}
			err := ToObject(bs, &actual)
			Expect(err).To(BeNil())
			Expect(actual).To(Equal(m))
		})

		It("should return expected bytes", func() {
			expected := []uint8{0x81, 0xa5, 0x73, 0x74, 0x75, 0x66, 0x66, 0xd0, 0x0a}
			Expect(expected).To(Equal(bs))
		})
	})

	Describe(".IsPathOk", func() {

		BeforeEach(func() {
			err := os.Mkdir("a_dir_here", 0655)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			err := os.Remove("a_dir_here")
			Expect(err).To(BeNil())
		})

		It("should return true when path exists", func() {
			Expect(IsPathOk("./a_dir_here")).To(BeTrue())
		})

		It("should return false when path does not exists", func() {
			Expect(IsPathOk("./abcxyz")).To(BeFalse())
		})
	})

	Describe(".IsFileOk", func() {

		BeforeEach(func() {
			err := os.Mkdir("a_dir_here", 0700)
			Expect(err).To(BeNil())
			err = ioutil.WriteFile("./a_dir_here/a_file", []byte("abc"), 0700)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			err := os.RemoveAll("a_dir_here")
			Expect(err).To(BeNil())
		})

		It("should return true when path exists", func() {
			Expect(IsFileOk("./a_dir_here/a_file")).To(BeTrue())
		})

		It("should return false when path does not exists", func() {
			Expect(IsFileOk("./abcxyz")).To(BeFalse())
		})
	})

	Describe(".NonZeroOrDefIn64", func() {
		It("should return 3 when v=0", func() {
			Expect(NonZeroOrDefIn64(0, 3)).To(Equal(int64(3)))
		})

		It("should return 2 when v=2", func() {
			Expect(NonZeroOrDefIn64(2, 3)).To(Equal(int64(2)))
		})
	})

	Describe(".NonZeroOrDefString", func() {
		It("should return def when v is empty", func() {
			Expect(NonZeroOrDefString("", "default")).To(Equal("default"))
		})

		It("should return val when v is non-empty", func() {
			Expect(NonZeroOrDefString("val", "def")).To(Equal("val"))
		})
	})

	Describe(".StrToDec", func() {
		It("should panic if value is not numeric", func() {
			val := "129.1a"
			Expect(func() {
				StrToDec(val)
			}).To(Panic())
		})
	})

	Describe(".Int64ToHex", func() {
		It("should return 0x3130", func() {
			Expect(Int64ToHex(10)).To(Equal("0x3130"))
		})
	})

	Describe(".HexToInt64", func() {
		It("should return 0x3130", func() {
			str, err := HexToInt64("0x3130")
			Expect(err).To(BeNil())
			Expect(str).To(Equal(int64(10)))
		})
	})

	Describe(".StrToHex", func() {
		It("should return 0x3130", func() {
			Expect(StrToHex("10")).To(Equal("0x3130"))
		})
	})

	Describe(".HexToStr", func() {
		It("should return '10'", func() {
			str, err := HexToStr("0x3130")
			Expect(err).To(BeNil())
			Expect(str).To(Equal("10"))
		})
	})

	Describe(".ToHex", func() {
		It("should return hex equivalent", func() {
			v := ToHex([]byte("abc"))
			Expect(v).To(Equal("0x616263"))
		})
	})

	Describe(".FromHex", func() {
		When("hex value begins with '0x'", func() {
			It("should return bytes equivalent of hex", func() {
				v, _ := FromHex("0x616263")
				Expect(v).To(Equal([]byte("abc")))
			})
		})

		When("hex value does not begin with '0x'", func() {
			It("should return bytes equivalent of hex", func() {
				v, _ := FromHex("616263")
				Expect(v).To(Equal([]byte("abc")))
			})
		})
	})

	Describe(".MustFromHex", func() {
		When("hex value begins with '0x'", func() {
			It("should return bytes equivalent of hex", func() {
				v := MustFromHex("0x616263")
				Expect(v).To(Equal([]byte("abc")))
			})
		})

		When("hex value is not valid'", func() {
			It("should panic", func() {
				Expect(func() {
					MustFromHex("sa&616263")
				}).To(Panic())
			})
		})
	})

	Describe(".RandString", func() {
		It("should produce string output of the specified length", func() {
			Expect(RandString(10)).To(HaveLen(10))
		})
	})

	Describe(".ToBasicMap", func() {

		type testStruct struct {
			Name string
		}

		It("should return correct map equivalent", func() {
			s := testStruct{Name: "odion"}
			expected := map[string]interface{}{"Name": "odion"}
			Expect(ToMap(s)).To(Equal(expected))
		})

	})

	Describe(".ToBasicMap", func() {

		type testStruct struct {
			Name string
			Age  CustomInt
		}

		It("should return correct map equivalent", func() {
			s := testStruct{Name: "odion", Age: 100}
			expected := map[string]interface{}{"Name": "odion", "Age": float64(100)}
			Expect(ToBasicMap(s)).To(Equal(expected))

			nonBasicExpected := map[string]interface{}{"Name": "odion", "Age": CustomInt(100)}
			Expect(ToMap(s)).To(Equal(nonBasicExpected))
		})
	})

	Describe(".StructSliceToMap", func() {

		type testStruct struct {
			Name string
		}

		It("should return correct map equivalent", func() {
			s := []testStruct{{Name: "odion"}, {Name: "Ken"}}
			expected := []Map{{"Name": "odion"}, {"Name": "Ken"}}
			Expect(StructSliceToMap(s)).To(Equal(expected))
		})
	})

	Describe(".GetPtrAddr", func() {
		It("should get numeric pointer address", func() {
			name := "xyz"
			ptrAddr := GetPtrAddr(name)
			Expect(ptrAddr.Cmp(Big0)).To(Equal(1))
		})
	})

	Describe(".DecodeMap", func() {

		type testStruct struct {
			Name string
		}

		It("should decode map to struct", func() {
			var m = map[string]interface{}{"Name": "abc"}
			var s testStruct
			err := DecodeMap(m, &s)
			Expect(err).To(BeNil())
			Expect(s.Name).To(Equal(m["Name"]))
		})
	})

	Describe(".EncodeNumber", func() {
		It("should encode number to expected byte", func() {
			encVal := EncodeNumber(100)
			Expect(encVal).To(Equal([]uint8{
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64,
			}))
		})
	})

	Describe(".DecodeNumber", func() {
		It("should decode bytes value to 100", func() {
			decVal := DecodeNumber([]uint8{
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64,
			})
			Expect(decVal).To(Equal(uint64(100)))
		})

		It("should panic if unable to decode", func() {
			Expect(func() {
				DecodeNumber([]byte("n10a"))
			}).To(Panic())
		})
	})

	Describe(".MayDecodeNumber", func() {
		It("should decode bytes value to 100", func() {
			decVal, err := MayDecodeNumber([]uint8{
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64,
			})
			Expect(err).To(BeNil())
			Expect(decVal).To(Equal(uint64(100)))
		})

		It("should return error if unable to decode", func() {
			_, err := MayDecodeNumber([]byte("n10a"))
			Expect(err).ToNot(BeNil())
		})
	})

	Describe(".IsBoolChanClosed", func() {
		It("should return false if bool channel is not closed", func() {
			c := make(chan bool)
			Expect(IsBoolChanClosed(c)).To(BeFalse())
		})

		It("should return true if bool channel is closed", func() {
			c := make(chan bool)
			close(c)
			Expect(IsBoolChanClosed(c)).To(BeTrue())
		})
	})

	Describe(".IsStructChanClosed", func() {
		It("should return false if struct channel is not closed", func() {
			c := make(chan struct{})
			Expect(IsStructChanClosed(c)).To(BeFalse())
		})

		It("should return true if struct channel is closed", func() {
			c := make(chan struct{})
			close(c)
			Expect(IsStructChanClosed(c)).To(BeTrue())
		})
	})

	Describe(".IsStructChanClosed", func() {
		It("should return false if struct channel is not closed", func() {
			c := make(chan func())
			Expect(IsFuncChanClosed(c)).To(BeFalse())
		})

		It("should return true if struct channel is closed", func() {
			c := make(chan func())
			close(c)
			Expect(IsFuncChanClosed(c)).To(BeTrue())
		})
	})

	Describe(".VMSet", func() {
		It("should successfully set object in vm context", func() {
			vm := otto.New()
			m := map[string]interface{}{"a": 2}
			val := VMSet(vm, "m", m)
			Expect(val).To(Equal(m))
			obj, err := vm.Object("m")
			Expect(err).To(BeNil())
			Expect(obj).ToNot(BeNil())
			m2, err := obj.Value().Export()
			Expect(err).To(BeNil())
			Expect(m2).To(Equal(m))
		})

		It("should not reset variable if already set", func() {
			vm := otto.New()
			m := map[string]interface{}{"a": 2}
			initial := VMSet(vm, "m", m)
			current := VMSet(vm, "m", map[string]interface{}{"a": 3})
			Expect(initial).To(Equal(current))
		})
	})

	Describe("Interrupt", func() {
		Describe(".CloseIsTrue", func() {
			It("should return true when closed and false when not", func() {
				itr := Interrupt(make(chan struct{}))
				Expect(itr.IsClosed()).To(BeFalse())
				close(itr)
				Expect(itr.IsClosed()).To(BeTrue())
			})
		})

		Describe(".Close", func() {
			It("should close the channel", func() {
				itr := Interrupt(make(chan struct{}))
				itr.Close()
				Expect(itr.IsClosed()).To(BeTrue())
			})
		})
	})

	Describe(".CopyMap", func() {
		It("should copy a map to another map", func() {
			src := map[string]interface{}{"age": 10}
			dst := make(map[string]interface{})
			CopyMap(src, dst)
			Expect(src).To(Equal(dst))
		})
	})

	Describe(".CloneMap", func() {
		It("should clone a map", func() {
			src := map[string]interface{}{"age": 10}
			clone := CloneMap(src)
			Expect(src).To(Equal(clone))
		})
	})

	Describe(".WriteJSON", func() {
		It("should write JSON to writer", func() {
			w := httptest.NewRecorder()
			WriteJSON(w, 500, map[string]interface{}{"age": 100})
			Expect(strings.TrimSpace(w.Body.String())).To(Equal(`{"age":100}`))
		})
	})

	Describe(".RESTApiErrorMsg", func() {
		It("should return an object with expected fields", func() {
			err := RESTApiErrorMsg("message", "field", "E200")
			Expect(err).To(HaveKey("error"))
			err = err["error"].(map[string]interface{})
			Expect(err["msg"]).To(Equal("message"))
			Expect(err["field"]).To(Equal("field"))
			Expect(err["code"]).To(Equal("E200"))
		})
	})

	Describe(".ToStringMapInter", func() {
		It("should convert map with non-interface value to map[string]interface{} type", func() {
			src := map[string]int{"jin": 20}
			out := ToStringMapInter(src)
			Expect(out).To(HaveLen(1))
			Expect(out["jin"]).To(Equal(20))
		})

		It("should panic if arg is not a map[string]interface{}", func() {
			src := map[int]string{1: "abc"}
			Expect(func() { ToStringMapInter(src) }).To(Panic())
		})

		It("should return same arg if arg is already a map[string]interface{}", func() {
			src := map[string]interface{}{"key": "abc"}
			out := ToStringMapInter(src)
			Expect(fmt.Sprintf("%p", src)).To(Equal(fmt.Sprintf("%p", out)))
		})

		It("should convert struct element to map if structToMap is true", func() {
			src := map[string]struct{ Name string }{"jin": {Name: "jin"}}
			out := ToStringMapInter(src, true)
			Expect(out).To(Equal(map[string]interface{}{"jin": map[string]interface{}{"Name": "jin"}}))
		})
	})

	Describe("IsZeroString", func() {
		It("should return true when value is empty or '0' or false when not", func() {
			Expect(IsZeroString("")).To(BeTrue())
			Expect(IsZeroString("0")).To(BeTrue())
			Expect(IsZeroString("1")).To(BeFalse())
		})
	})

	Describe(".XorBytes", func() {
		It("should return '4' for '6' and '2'", func() {
			r := XorBytes([]byte("6"), []byte("2"))
			Expect(r).To(Equal([]uint8{0x04}))
		})
	})

	Describe(".RemoveFlag", func() {
		Specify("case 1", func() {
			res := RemoveFlag([]string{"-nickname", "abc", "--age", "12"}, []string{"name", "height"})
			Expect(res).To(Equal([]string{"-nickname", "abc", "--age", "12"}))
		})
		Specify("case 2", func() {
			res := RemoveFlag([]string{"--nickname", "abc", "--age", "12"}, []string{"name", "age"})
			Expect(res).To(Equal([]string{"--nickname", "abc"}))
		})
		Specify("case 3", func() {
			res := RemoveFlag([]string{"--nickname", "abc", "--age", "12"}, []string{"nickname", "height"})
			Expect(res).To(Equal([]string{"--age", "12"}))
		})

		Specify("case 4", func() {
			res := RemoveFlag([]string{"--nickname=abc", "--age", "12"}, []string{"nickname", "height"})
			Expect(res).To(Equal([]string{"--age", "12"}))
		})

		Specify("case 5", func() {
			res := RemoveFlag([]string{"--nickname", "logan", "xmen"}, []string{"nickname"})
			Expect(res).To(Equal([]string{"xmen"}))
		})

		Specify("case 6", func() {
			res := RemoveFlag([]string{"--nickname", "logan", "xmen", "--sex", "female"}, []string{"nickname"})
			Expect(res).To(Equal([]string{"xmen", "--sex", "female"}))
		})
	})

	Describe(".ParseExtArgs", func() {
		It("case 1", func() {
			extArgs := map[string]string{"e1.size": "200", "e2.phase": "start"}
			res, common := ParseExtArgs(extArgs)
			Expect(res).To(Equal(map[string]map[string]string{
				"e1": {"size": "200"},
				"e2": {"phase": "start"},
			}))
			Expect(common).To(BeEmpty())
		})

		It("case 2 - with a common argument", func() {
			extArgs := map[string]string{"e1.size": "200", "e2.phase": "start", "env": "dev"}
			res, common := ParseExtArgs(extArgs)
			Expect(res).To(Equal(map[string]map[string]string{
				"e1": {"size": "200"},
				"e2": {"phase": "start"},
			}))
			Expect(common).To(Equal(map[string]string{"env": "dev"}))
		})

		It("case 3 - with a non-unique common argument ", func() {
			extArgs := map[string]string{"e1.size": "200", "e2.phase": "start", "size": "100"}
			res, common := ParseExtArgs(extArgs)
			Expect(res).To(Equal(map[string]map[string]string{
				"e1": {"size": "200"},
				"e2": {"phase": "start"},
			}))
			Expect(common).To(Equal(map[string]string{"size": "100"}))
		})
	})

	Describe(".SplitNamespaceDomain", func() {
		When("address format is not valid", func() {
			It("should return error", func() {
				_, _, err := SplitNamespaceDomain("/some/kind/of/path")
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("invalid address format"))
			})
		})

		When("address format is valid", func() {
			It("should return no error, namespace and domain", func() {
				ns, domain, err := SplitNamespaceDomain("coinfiddle/payment")
				Expect(err).To(BeNil())
				Expect(ns).To(Equal("coinfiddle"))
				Expect(domain).To(Equal("payment"))
			})
		})
	})

	Describe(".IsString", func() {
		It("should return true if string or false if not", func() {
			Expect(IsString("abc")).To(BeTrue())
			Expect(IsString(123)).To(BeFalse())
		})
	})

	Describe(".RemoveFromStringSlice", func() {
		It("should return [a,c] when b is removed from [a,b,c]", func() {
			Expect(RemoveFromStringSlice([]string{"a", "b", "c"}, "b")).To(Equal([]string{"a", "c"}))
		})
		It("should return [a,b,c] when d is supposed to be removed from [a,b,c]", func() {
			Expect(RemoveFromStringSlice([]string{"a", "b", "c"}, "d")).To(Equal([]string{"a", "b", "c"}))
		})
	})

	Describe(".ParseContentFrontMatter", func() {
		It("should return content only when content does not contain front matter", func() {
			str := "content only"
			cfm, err := ParseContentFrontMatter(strings.NewReader(str))
			Expect(err).To(BeNil())
			Expect(cfm.Content).To(Equal([]byte(str)))
		})

		It("should return content only when content does not contain front matter (case 2)", func() {
			str := "co"
			cfm, err := ParseContentFrontMatter(strings.NewReader(str))
			Expect(err).To(BeNil())
			Expect(cfm.Content).To(Equal([]byte(str)))
		})

		It("should return nil content when content is malformed", func() {
			str := "---\nco"
			cfm, err := ParseContentFrontMatter(strings.NewReader(str))
			Expect(err).To(BeNil())
			Expect(cfm.Content).To(BeNil())
		})

		It("should return front matter and content", func() {
			str := "---\nage: 1000\n---\nxyz"
			cfm, err := ParseContentFrontMatter(strings.NewReader(str))
			Expect(err).To(BeNil())
			Expect(cfm.Content).To(Equal([]byte("xyz")))
			Expect(cfm.FrontMatter).To(Equal(map[string]interface{}{
				"age": 1000,
			}))
		})
	})

	Describe(".IsMapOrStruct", func() {
		It("should check for true/false", func() {
			Expect(IsMapOrStruct(map[string]int{})).To(BeTrue())
			Expect(IsMapOrStruct(map[string]interface{}{})).To(BeTrue())
			Expect(IsMapOrStruct(struct{}{})).To(BeTrue())
			Expect(IsMapOrStruct(&struct{}{})).To(BeTrue())
		})
	})

	Describe(".ToByteSlice", func() {
		It("should convert to byte slice", func() {
			iSlice := []int{67, 155, 203, 212, 6, 5, 51, 24, 234, 106, 169, 130, 184, 236, 235, 83, 105, 122, 64, 91, 11, 54, 228, 7, 143, 69, 81, 88, 62, 96, 19, 203}
			bSlice := []byte{67, 155, 203, 212, 6, 5, 51, 24, 234, 106, 169, 130, 184, 236, 235, 83, 105, 122, 64, 91, 11, 54, 228, 7, 143, 69, 81, 88, 62, 96, 19, 203}
			Expect(ToByteSlice(iSlice)).To(Equal(bSlice))
		})
	})

	Describe(".ParseGitVersion", func() {
		It("should return expected values", func() {
			Expect(ParseGitVersion("git version 2.26.2.windows.1")).To(Equal("2.26.2"))
			Expect(ParseGitVersion("git version 2.26.2")).To(Equal("2.26.2"))
			Expect(ParseGitVersion("git version 2.26")).To(Equal("2.26"))
			Expect(ParseGitVersion("git version 2")).To(Equal("2"))
		})
	})
})
