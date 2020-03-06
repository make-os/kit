package util

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/util/mocks"
)

var _ = Describe("SerializerHelper", func() {
	var ctrl *gomock.Controller
	var sh *SerializerHelper

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		sh = &SerializerHelper{}
	})

	Describe(".DecodeMulti", func() {
		When("underlying DecodeMulti returns err that is not 'EOF'", func() {
			It("should not return nil", func() {
				mockDecoder := mocks.NewMockDecoder(ctrl)
				mockDecoder.EXPECT().DecodeMulti(gomock.Any()).Return(fmt.Errorf("bad error"))
				err := sh.DecodeMulti(mockDecoder, 1)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("bad error"))
			})
		})

		When("underlying DecodeMulti returns err that is 'EOF'", func() {
			It("should return nil", func() {
				mockDecoder := mocks.NewMockDecoder(ctrl)
				mockDecoder.EXPECT().DecodeMulti(gomock.Any()).Return(fmt.Errorf("EOF"))
				err := sh.DecodeMulti(mockDecoder, 1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".EncodeMulti", func() {
		When("map with non-string|interface{} value is passed", func() {
			It("should be converted to map[string]interface{}", func() {
				mockEncoder := mocks.NewMockEncoder(ctrl)
				mockEncoder.EXPECT().EncodeMulti(gomock.AssignableToTypeOf(map[string]interface{}{}), 3).Return(nil)
				err := sh.EncodeMulti(mockEncoder, map[string]int{"age": 100}, 3)
				Expect(err).To(BeNil())
			})
		})
	})
})
