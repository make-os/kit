package types_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/makeos/mosdef/types"
)

var _ = Describe("Namespace", func() {
	Describe(".IsNil", func() {
		It("should return true when all fields have zero values", func() {
			ns := BareNamespace()
			Expect(ns.IsNil()).To(BeTrue())
		})

		It("should return false when not all fields have zero values", func() {
			ns := BareNamespace()
			ns.Owner = "addr"
			Expect(ns.IsNil()).To(BeFalse())
		})
	})

	Describe(".Bytes", func() {
		It("should return non-empty bytes slice", func() {
			ns := BareNamespace()
			ns.Owner = "addr"
			Expect(ns.Bytes()).ToNot(BeEmpty())
		})
	})

	Describe(".NewNamespaceFromBytes", func() {
		It("should recreate Namespace object from bytes", func() {
			ns := BareNamespace()
			ns.Owner = "owner"
			ns.ExpiresAt = 20
			ns.GraceEndAt = 15
			ns.Domains = map[string]string{"name": "target"}
			bz := ns.Bytes()
			ns2, err := NewNamespaceFromBytes(bz)
			Expect(err).To(BeNil())
			Expect(ns).To(Equal(ns2))
		})

		Context("with nil target", func() {
			It("should recreate Namespace object from bytes", func() {
				ns := BareNamespace()
				ns.Owner = "owner"
				ns.ExpiresAt = 20
				ns.GraceEndAt = 15
				ns.Domains = nil
				bz := ns.Bytes()
				ns2, err := NewNamespaceFromBytes(bz)
				Expect(err).To(BeNil())
				Expect(ns).To(Equal(ns2))
			})
		})
	})
})
