package common

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Types", func() {
	Describe("NewRecord.GetKey", func() {
		It("should create key", func() {
			r := NewRecord([]byte("age"), []byte("20"), []byte("prefix"))
			key := r.GetKey()
			Expect(key).ToNot(BeEmpty())
			Expect(key).To(Equal([]byte("prefix;age")))
		})
	})

	Describe("NewRecord.IsEmpty", func() {
		It("should return true when empty", func() {
			r := NewRecord([]byte(""), []byte(""), nil)
			empty := r.IsEmpty()
			Expect(empty).To(BeTrue())
		})

		It("should return false when not empty", func() {
			r := NewRecord([]byte("abc"), []byte("xyz"), nil)
			empty := r.IsEmpty()
			Expect(empty).To(BeFalse())
		})
	})

	Describe(".NewFromKeyValue", func() {
		When("key has no KeyPrefixSeparator", func() {
			It("should return object that contains key=age, prefix= value=20 and GetKey=age;", func() {
				o := NewFromKeyValue([]byte("age"), []byte("20"))
				Expect(o.Prefix).To(BeEmpty())
				Expect(o.Key).To(Equal([]byte("age")))
				Expect(o.Value).To(Equal([]byte("20")))
				Expect(o.GetKey()).To(Equal([]byte("age")))
			})
		})

		When("key has KeyPrefixSeparator", func() {
			It("should return object that contains key=age, value=20, prefix=prefixA", func() {
				o := NewFromKeyValue([]byte("prefixA;age"), []byte("20"))
				Expect(o.Key).To(Equal([]byte("age")))
				Expect(o.Value).To(Equal([]byte("20")))
				Expect(o.GetKey()).To(Equal([]byte("prefixA;age")))
				Expect(o.Prefix).To(Equal([]byte("prefixA")))
			})
		})
	})

	Describe(".MakePrefix", func() {
		It("should return 'prefixA:prefixB'", func() {
			actual := MakePrefix([]byte("prefixA"), []byte("prefixB"))
			Expect(string(actual)).To(Equal("prefixA:prefixB"))
		})

		It("should return 'prefixA'", func() {
			actual := MakePrefix([]byte("prefixA"))
			Expect(string(actual)).To(Equal("prefixA"))
		})

		It("should return empty string when prefixes are not provided", func() {
			actual := MakePrefix()
			Expect(string(actual)).To(Equal(""))
		})
	})

	Describe(".MakeKey", func() {
		It("should return 'prefixA:prefixB;age' when key and prefixes are provided", func() {
			actual := MakeKey([]byte("age"), []byte("prefixA"), []byte("prefixB"))
			Expect(string(actual)).To(Equal("prefixA:prefixB;age"))
		})

		It("should return only concatenated prefixes 'prefixA:prefixB' with no KeyPrefixSeparator when key is not provided", func() {
			actual := MakeKey(nil, []byte("prefixA"), []byte("prefixB"))
			Expect(string(actual)).To(Equal("prefixA:prefixB"))
		})

		It("should return only key with no KeyPrefixSeparator when prefixes are not provided", func() {
			actual := MakeKey([]byte("age"), nil)
			Expect(string(actual)).To(Equal("age"))
		})

		It("should return empty string when key and prefixes are not provided", func() {
			actual := MakeKey(nil, nil)
			Expect(string(actual)).To(Equal(""))
		})
	})

	Describe("NewRecord.GetKey vs .MakeKey", func() {
		It("should return same result when key and prefixes are provided", func() {
			r := NewRecord([]byte("age"), []byte("value"), []byte("prefixA"), []byte("prefixB"))
			key := MakeKey([]byte("age"), []byte("prefixA"), []byte("prefixB"))
			Expect(r.GetKey()).To(Equal(key))
		})

		It("should return same result when key is not provided", func() {
			r := NewRecord(nil, []byte("value"), []byte("prefixA"), []byte("prefixB"))
			key := MakeKey(nil, []byte("prefixA"), []byte("prefixB"))
			Expect(r.GetKey()).To(Equal(key))
		})

		It("should return same result when prefixes are not provided", func() {
			r := NewRecord([]byte("age"), nil)
			key := MakeKey([]byte("age"), nil)
			Expect(r.GetKey()).To(Equal(key))
		})

		It("should return empty string when key and prefixes are not provided", func() {
			r := NewRecord(nil, nil)
			key := MakeKey(nil, nil)
			Expect(r.GetKey()).To(Equal(key))
		})
	})

})
