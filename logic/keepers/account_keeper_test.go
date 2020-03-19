package keepers

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
	"gitlab.com/makeos/mosdef/pkgs/tree"
	state2 "gitlab.com/makeos/mosdef/types/state"

	"gitlab.com/makeos/mosdef/util"
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
				acct := ak.Get(util.Address("unknown"), 0)
				Expect(acct).To(Equal(state2.BareAccount()))
			})
		})

		When("account exists on the latest block", func() {
			var testAcct = state2.BareAccount()

			BeforeEach(func() {
				testAcct.Nonce = 1
				testAcct.Balance = util.String("100")
				acctKey := MakeAccountKey("addr1")
				state.Set(acctKey, testAcct.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should successfully return the expected account object", func() {
				acct := ak.Get("addr1", 0)
				Expect(acct).To(BeEquivalentTo(testAcct))
			})
		})
	})

	Describe(".Update", func() {
		It("should update balance", func() {
			key := util.Address("addr1")
			acct := ak.Get(key)
			Expect(acct.Balance).To(Equal(util.String("0")))
			Expect(acct.Nonce).To(Equal(uint64(0)))
			acct.Balance = util.String("10000")
			acct.Nonce = 2
			ak.Update(key, acct)
			acct = ak.Get(key)
			Expect(acct.Balance).To(Equal(util.String("10000")))
			Expect(acct.Nonce).To(Equal(uint64(2)))
		})
	})
})
