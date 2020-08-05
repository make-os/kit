package keepers

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
	"github.com/themakeos/lobe/pkgs/tree"
	state2 "github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/util/crypto"
)

var _ = Describe("NamespaceKeeper", func() {
	var state *tree.SafeTree
	var nsKp *NamespaceKeeper

	BeforeEach(func() {
		state = tree.NewSafeTree(tmdb.NewMemDB(), 128)
		nsKp = NewNamespaceKeeper(state)
	})

	Describe(".Get", func() {
		When("namespace does not exist", func() {
			It("should return a bare namespace", func() {
				ns := nsKp.Get("unknown", 0)
				Expect(ns).To(Equal(state2.BareNamespace()))
			})
		})

		When("namespace exists", func() {
			var testNS = state2.BareNamespace()

			BeforeEach(func() {
				testNS.Owner = "creator_addr"
				nsKey := MakeNamespaceKey("ns1")
				state.Set(nsKey, testNS.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should successfully return the expected namespace object", func() {
				ns := nsKp.Get("ns1", 0)
				Expect(ns).To(BeEquivalentTo(testNS))
			})
		})
	})

	Describe(".Update", func() {
		It("should update namespace object", func() {
			key := "repo1"
			ns := nsKp.Get(key)
			Expect(ns.Owner).To(Equal(""))

			ns.Owner = "creator_addr"
			nsKp.Update(key, ns)

			ns2 := nsKp.Get(key)
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
			var testNS = state2.BareNamespace()

			BeforeEach(func() {
				testNS.Owner = "creator_addr"
				nsKey := MakeNamespaceKey(crypto.MakeNamespaceHash("ns1"))
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
			var testNS = state2.BareNamespace()

			BeforeEach(func() {
				testNS.Owner = "creator_addr"
				testNS.Domains["domain"] = "target1"

				nsKey := MakeNamespaceKey(crypto.MakeNamespaceHash("ns1"))
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
