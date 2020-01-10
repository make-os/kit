package jsmodules_test

import (
	"math/big"

	"github.com/makeos/mosdef/jsmodules"
	"github.com/makeos/mosdef/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Common", func() {

	Describe(".EncodeForJS", func() {

		type test1 struct {
			Name string
			Desc []byte
		}

		type test2 struct {
			Age    int
			Others test1
			More   []interface{}
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

		It("should return expected output", func() {
			t1 := test1{
				Name: "fred",
				Desc: []byte("i love games"),
			}
			result := jsmodules.EncodeForJS(t1)
			Expect(result).To(Equal(map[string]interface{}{"Name": "fred",
				"Desc": "0x69206c6f76652067616d6573",
			}))

			t2 := test2{
				Age:    20,
				Others: t1,
			}
			result = jsmodules.EncodeForJS(t2)
			Expect(result.(map[string]interface{})["Others"]).To(Equal(map[string]interface{}{
				"Name": "fred",
				"Desc": "0x69206c6f76652067616d6573",
			}))

			t3 := test2{
				Age:    20,
				Others: t1,
				More:   []interface{}{t1},
			}
			result = jsmodules.EncodeForJS(t3)
			Expect(result.(map[string]interface{})["More"]).To(Equal([]interface{}{
				map[string]interface{}{"Name": "fred",
					"Desc": "0x69206c6f76652067616d6573",
				},
			}))

			t4 := test3{
				Sig: util.StrToBytes32("fred"),
			}
			result = jsmodules.EncodeForJS(t4)
			Expect(result).To(Equal(map[string]interface{}{"Sig": "0x6672656400000000000000000000000000000000000000000000000000000000"}))

			t5 := test4{
				Num: new(big.Int).SetInt64(10),
			}
			result = jsmodules.EncodeForJS(t5)
			Expect(result).To(Equal(map[string]interface{}{"Num": "10"}))

			t6 := test5{
				Num: util.EncodeNonce(10),
			}
			result = jsmodules.EncodeForJS(t6)
			Expect(result).To(Equal(map[string]interface{}{"Num": "0x000000000000000a"}))
		})

		Context("With ignoreField specified", func() {
			t1 := test2{Age: 30, Others: test1{Desc: []byte("i love games")}}

			BeforeEach(func() {
				result := jsmodules.EncodeForJS(t1)
				Expect(result.(map[string]interface{})["Age"]).To(Equal("30"))
			})

			It("should not modify field", func() {
				result := jsmodules.EncodeForJS(t1, "Age")
				Expect(result.(map[string]interface{})["Age"]).To(Equal(30))
			})
		})
	})

})
