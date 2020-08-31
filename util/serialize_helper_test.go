package util

import (
	"bytes"
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/util/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmihailenco/msgpack"
)

type testStruct struct {
	CodecUtil
	Name string
}

func (t *testStruct) EncodeMsgpack(encoder *msgpack.Encoder) error {
	return t.EncodeMulti(encoder, t.Name)
}

func (t *testStruct) DecodeMsgpack(decoder *msgpack.Decoder) error {
	return t.DecodeMulti(decoder, &t.Name)
}

var _ = Describe("CodecUtil", func() {
	var ctrl *gomock.Controller
	var sh *CodecUtil

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		sh = &CodecUtil{}
	})

	Describe(".DecodeMulti", func() {
		When("non-EOF error is returned", func() {
			It("should return the non-EOF error", func() {
				mockDecoder := mocks.NewMockDecoder(ctrl)
				mockDecoder.EXPECT().DecodeMulti(gomock.Any()).Return(fmt.Errorf("bad error"))

				sh.DecodedVersion = "v1"
				err := sh.DecodeMulti(mockDecoder, 1)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("bad error"))
			})
		})

		When("version has been decoded and EOF is returned", func() {
			It("should return nil", func() {
				mockDecoder := mocks.NewMockDecoder(ctrl)
				sh.versionDecoded = true
				mockDecoder.EXPECT().DecodeMulti(gomock.Any()).Return(fmt.Errorf("EOF"))
				err := sh.DecodeMulti(mockDecoder, 1)
				Expect(err).To(BeNil())
			})
		})

		When("version has not been decoded and EOF is returned", func() {
			It("should return EOF", func() {
				mockDecoder := mocks.NewMockDecoder(ctrl)
				mockDecoder.EXPECT().DecodeMulti(gomock.Any()).Return(fmt.Errorf("EOF"))
				err := sh.DecodeMulti(mockDecoder, 1)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("EOF"))
			})
		})
	})

	Describe(".EncodeMulti", func() {
		When("map with value type is not a string or interface{}", func() {
			It("should be converted to map[string]interface{}", func() {
				mockEncoder := mocks.NewMockEncoder(ctrl)
				mockEncoder.EXPECT().EncodeMulti("").Return(nil)
				mockEncoder.EXPECT().EncodeMulti(gomock.AssignableToTypeOf(map[string]interface{}{})).Return(nil)
				err := sh.EncodeMulti(mockEncoder, map[string]int{"age": 100})
				Expect(err).To(BeNil())
			})
		})

		When("[]int8 slice is nil", func() {
			It("should be converted to map[string]interface{}", func() {
				mockEncoder := mocks.NewMockEncoder(ctrl)
				mockEncoder.EXPECT().EncodeMulti("").Return(nil)
				mockEncoder.EXPECT().EncodeMulti(gomock.AssignableToTypeOf([]uint8{})).Return(nil)
				var x []uint8
				err := sh.EncodeMulti(mockEncoder, x)
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Encode and decode an object with a codec version", func() {
		It("should correctly encode and decode the object", func() {
			obj := &testStruct{CodecUtil: CodecUtil{Version: "1.2"}, Name: "Ben"}
			bz := ToBytes(obj)
			var obj2 testStruct
			err := ToObject(bz, &obj2)
			Expect(err).To(BeNil())
			Expect(obj2.Version).To(Equal(""))
			Expect(obj2.Name).To(Equal("Ben"))
			Expect(obj2.DecodedVersion).To(Equal("1.2"))
		})
	})

	Describe(".DecodeVersion", func() {
		var err error
		var version string
		var dec *msgpack.Decoder
		obj := &testStruct{CodecUtil: CodecUtil{Version: "1.2"}, Name: "Ben"}

		BeforeEach(func() {
			bz := ToBytes(obj)
			dec = msgpack.NewDecoder(bytes.NewReader(bz))
			version, err = obj.DecodeVersion(dec)
		})

		It("should correctly get codec version", func() {
			Expect(err).To(BeNil())
			Expect(version).To(Equal(obj.Version))
		})

		Context("Calling DecodeMulti after calling DecodeVersion", func() {
			It("should correctly decoded object", func() {
				var obj2 = testStruct{}
				err := obj.DecodeMulti(dec, &obj2.Name)
				Expect(err).To(BeNil())
				Expect(obj2.Name).To(Equal(obj.Name))
			})
		})
	})
})
