package modules_test

import (
	"math/big"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/modules"
	"gitlab.com/makeos/mosdef/util"
)

type TestCase struct {
	Desc           string
	Obj            interface{}
	Expected       interface{}
	FieldsToIgnore []string
	ShouldPanic    bool
}

var _ = Describe("Common", func() {

	Describe(".Normalize", func() {

		type test1 struct {
			Name string
			Desc []byte
		}

		type test2 struct {
			Age    int64
			Others test1
			More   []interface{} `json:",omitempty"`
		}

		type test3 struct {
			Sig util.Bytes32
		}

		type test4 struct {
			Num *big.Int
		}

		type test5 struct {
			Num util.BlockNonce
		}

		var t1 = test1{Name: "fred", Desc: []byte("i love games")}
		var cases = []TestCase{
			{
				Desc:        "should panic with non-map, non-slice map or struct",
				Obj:         []interface{}{1, 2},
				ShouldPanic: true,
			},
			{
				Desc:        "should panic with nil result",
				Obj:         nil,
				Expected:    []util.Map{},
				ShouldPanic: true,
			},
			{
				Desc:     "with a string and []byte field",
				Obj:      t1,
				Expected: util.Map{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"},
			},
			{
				Desc:     "with an integer field and a struct field (with string and []byte fields)",
				Obj:      test2{Age: 20, Others: t1},
				Expected: util.Map{"Age": "20", "Others": util.Map{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"}},
			},
			{
				Desc: "with an integer field, a slice of struct field and a struct field (with string and []byte fields)",
				Obj:  test2{Age: 20, Others: t1, More: []interface{}{t1}},
				Expected: util.Map{"Age": "20",
					"Others": util.Map{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"},
					"More":   []interface{}{util.Map{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"}}},
			},
			{
				Desc:     "with a byte slice field",
				Obj:      test3{Sig: util.StrToBytes32("fred")},
				Expected: util.Map{"Sig": "0x6672656400000000000000000000000000000000000000000000000000000000"},
			},
			{
				Desc:     "with a big.Int field",
				Obj:      test4{Num: new(big.Int).SetInt64(10)},
				Expected: util.Map{"Num": "10"},
			},
			{
				Desc:     "with a BlockNonce field",
				Obj:      test5{Num: util.EncodeNonce(10)},
				Expected: util.Map{"Num": "0x000000000000000a"},
			},
			{
				Desc:           "with fields to be ignored",
				Obj:            test2{Age: 30, Others: test1{Desc: []byte("i love games")}},
				FieldsToIgnore: []string{"Age"},
				Expected:       util.Map{"Age": int64(30), "Others": util.Map{"Name": "", "Desc": "0x69206c6f76652067616d6573"}},
			},
			{
				Desc: "with a slice of structs",
				Obj:  []interface{}{t1, t1},
				Expected: []util.Map{
					{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"},
					{"Name": "fred", "Desc": "0x69206c6f76652067616d6573"},
				},
			},
			{
				Desc: "with a slice of map[string]int",
				Obj:  []interface{}{map[string]int{"age": 10}, map[string]int{"age": 1000}},
				Expected: []util.Map{
					{"age": 10},
					{"age": 1000},
				},
			},
			{
				Desc: "with a slice of map[string]float64",
				Obj:  []interface{}{map[string]float64{"age": 10.2}, map[string]float64{"age": 1000.3}},
				Expected: []util.Map{
					{"age": "10.2"},
					{"age": "1000.3"},
				},
			},
			{
				Desc:        "should panic with non-map, non-slice map or struct",
				Obj:         "string",
				Expected:    []util.Map{},
				ShouldPanic: true,
			},
		}

		for _, c := range cases {
			_c := c
			It(_c.Desc, func() {
				if !_c.ShouldPanic {
					result := modules.Normalize(_c.Obj, _c.FieldsToIgnore...)
					Expect(result).To(Equal(_c.Expected))
				} else {
					Expect(func() {
						modules.Normalize(_c.Obj, _c.FieldsToIgnore...)
					}).To(Panic())
				}
			})
		}
	})
})
