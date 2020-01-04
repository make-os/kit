package keepers

import (
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/types"
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

		When("repository exists", func() {
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
})
