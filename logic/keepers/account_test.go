package keepers

import (
	"github.com/make-os/lobe/pkgs/tree"
	state2 "github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/util/identifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"

	"github.com/make-os/lobe/util"
)

var _ = Describe("Account", func() {
	var state *tree.SafeTree
	var ak *AccountKeeper

	BeforeEach(func() {
		state = tree.NewSafeTree(tmdb.NewMemDB(), 128)
		ak = NewAccountKeeper(state)
	})

	Describe(".Get", func() {
		When("account does not exist", func() {
			It("should return a bare account", func() {
				acct := ak.Get("unknown", 0)
				Expect(acct).To(Equal(state2.BareAccount()))
			})
		})

		When("account exists on the latest block", func() {
			var testAcct = state2.BareAccount()

			BeforeEach(func() {
				testAcct.Nonce = 1
				testAcct.Balance = "100"
				acctKey := MakeAccountKey("addr1")
				state.Set(acctKey, testAcct.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should successfully return the expected account object", func() {
				acct := ak.Get("addr1", 0)
				Expect(acct.Bytes()).To(Equal(testAcct.Bytes()))
			})
		})
	})

	Describe(".Update", func() {
		It("should update balance", func() {
			key := identifier.Address("addr1")
			acct := ak.Get(key)
			Expect(acct.Balance).To(Equal(util.String("0")))
			Expect(acct.Nonce.UInt64()).To(Equal(uint64(0)))
			acct.Balance = "10000"
			acct.Nonce = 2
			ak.Update(key, acct)
			acct = ak.Get(key)
			Expect(acct.Balance).To(Equal(util.String("10000")))
			Expect(acct.Nonce.UInt64()).To(Equal(uint64(2)))
		})
	})
})
