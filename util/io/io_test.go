package io

import (
	"io/ioutil"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IO", func() {
	Describe(".LimitedReadToTmpFile", func() {
		It("should return ErrSrcTooLarge when limit is 5 and reader contains 'Hello World'", func() {
			rdr := strings.NewReader("Hello World")
			r, err := LimitedReadToTmpFile(rdr, 5)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrSrcTooLarge))
			Expect(r).To(BeNil())
		})

		It("should return 'Hello World' when limit is 11 and reader contains 'Hello World'", func() {
			rdr := strings.NewReader("Hello World")
			r, err := LimitedReadToTmpFile(rdr, 11)
			Expect(err).To(BeNil())
			data, err := ioutil.ReadAll(r)
			Expect(string(data)).To(Equal("Hello World"))
		})

		It("should return 'Hello' when limit is 11 and reader contains 'Hello'", func() {
			rdr := strings.NewReader("Hello")
			r, err := LimitedReadToTmpFile(rdr, 11)
			Expect(err).To(BeNil())
			data, err := ioutil.ReadAll(r)
			Expect(string(data)).To(Equal("Hello"))
		})
	})
})
