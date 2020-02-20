package keepers

import (
	"gitlab.com/makeos/mosdef/storage/tree"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

var _ = Describe("NamespaceKeeper", func() {
	var state *tree.SafeTree
	var nsKp *NamespaceKeeper

	BeforeEach(func() {
		state = tree.NewSafeTree(tmdb.NewMemDB(), 128)
		nsKp = NewNamespaceKeeper(state)
	})

	Describe(".GetNamespace", func() {
		When("namespace does not exist", func() {
			It("should return a bare namespace", func() {
				ns := nsKp.GetNamespace("unknown", 0)
				Expect(ns).To(Equal(types.BareNamespace()))
			})
		})

		When("namespace exists", func() {
			var testNS = types.BareNamespace()

			BeforeEach(func() {
				testNS.Owner = "creator_addr"
				nsKey := MakeNamespaceKey("ns1")
				state.Set(nsKey, testNS.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should successfully return the expected namespace object", func() {
				ns := nsKp.GetNamespace("ns1", 0)
				Expect(ns).To(BeEquivalentTo(testNS))
			})
		})
	})

	Describe(".Update", func() {
		It("should update namespace object", func() {
			key := "repo1"
			ns := nsKp.GetNamespace(key)
			Expect(ns.Owner).To(Equal(""))

			ns.Owner = "creator_addr"
			nsKp.Update(key, ns)

			ns2 := nsKp.GetNamespace(key)
			Expect(ns2).To(Equal(ns))
		})
	})

	Describe(".GetTarget", func() {
		When("path is not valid", func() {
			It("should return err", func() {
				_, err := nsKp.GetTarget("an/invalid/path", 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("invalid address format"))
			})
		})

		When("namespace does not exist", func() {
			It("should return err", func() {
				_, err := nsKp.GetTarget("unknown_namespace/domain", 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("namespace not found"))
			})
		})

		When("domain not found", func() {
			var testNS = types.BareNamespace()

			BeforeEach(func() {
				testNS.Owner = "creator_addr"
				nsKey := MakeNamespaceKey(util.Hash20Hex([]byte("ns1")))
				state.Set(nsKey, testNS.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should return err", func() {
				_, err := nsKp.GetTarget("ns1/domain", 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("domain not found"))
			})
		})

		When("domain exist", func() {
			var testNS = types.BareNamespace()

			BeforeEach(func() {
				testNS.Owner = "creator_addr"
				testNS.Domains["domain"] = "target1"

				nsKey := MakeNamespaceKey(util.Hash20Hex([]byte("ns1")))
				state.Set(nsKey, testNS.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should return err", func() {
				target, err := nsKp.GetTarget("ns1/domain", 0)
				Expect(err).To(BeNil())
				Expect(target).To(Equal("target1"))
			})
		})
	})
})
