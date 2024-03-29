package state_test

import (
	"github.com/make-os/kit/types/state"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Namespace", func() {
	Describe(".IsEmpty", func() {
		It("should return true when all fields have zero values", func() {
			ns := state.BareNamespace()
			Expect(ns.IsNil()).To(BeTrue())
		})

		It("should return false when not all fields have zero values", func() {
			ns := state.BareNamespace()
			ns.Owner = "addr"
			Expect(ns.IsNil()).To(BeFalse())
		})
	})

	Describe(".Bytes", func() {
		It("should return non-empty bytes slice", func() {
			ns := state.BareNamespace()
			ns.Owner = "addr"
			Expect(ns.Bytes()).ToNot(BeEmpty())
		})
	})

	Describe(".NewNamespaceFromBytes", func() {
		It("should recreate Namespace object from bytes", func() {
			ns := state.BareNamespace()
			ns.Owner = "owner"
			ns.ExpiresAt = 20
			ns.GraceEndAt = 15
			ns.Domains = map[string]string{"name": "target"}
			bz := ns.Bytes()
			ns2, err := state.NewNamespaceFromBytes(bz)
			Expect(err).To(BeNil())
			Expect(ns.Bytes()).To(Equal(ns2.Bytes()))
		})

		Context("with nil target", func() {
			It("should recreate Namespace object from bytes", func() {
				ns := state.BareNamespace()
				ns.Owner = "owner"
				ns.ExpiresAt = 20
				ns.GraceEndAt = 15
				ns.Domains = map[string]string{}
				bz := ns.Bytes()
				ns2, err := state.NewNamespaceFromBytes(bz)
				Expect(err).To(BeNil())
				Expect(ns.Bytes()).To(Equal(ns2.Bytes()))
			})
		})
	})
})
