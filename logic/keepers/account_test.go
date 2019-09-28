package keepers_test

import (
	"github.com/makeos/mosdef/storage/tree"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"

	keepers "github.com/makeos/mosdef/logic/keepers"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("Account", func() {
	var state *tree.SafeTree
	var ak *keepers.AccountKeeper

	BeforeEach(func() {
		state = tree.NewSafeTree(tmdb.NewMemDB(), 128)
		ak = keepers.NewAccountKeeper(state)
	})

	Describe(".GetAccount", func() {
		When("account does not exist", func() {
			It("should return a bare account", func() {
				acct := ak.GetAccount(util.String("unknown"), 0)
				Expect(acct.Balance).To(Equal(util.String("0")))
				Expect(acct.Nonce).To(Equal(uint64(0)))
			})
		})

		When("account exist on the latest block", func() {
			var testAcct = keepers.MakeBareAccount()

			BeforeEach(func() {
				testAcct.Nonce = 1
				testAcct.Balance = util.String("100")
				acctKey := keepers.MakeAccountKey("addr1")
				state.Set(acctKey, testAcct.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should successfully return the block", func() {
				acct := ak.GetAccount("addr1", 0)
				Expect(acct).To(BeEquivalentTo(testAcct))
			})
		})

		When("account exist on two different blocks", func() {
			var testAcct = keepers.MakeBareAccount()
			var testAcct2 = keepers.MakeBareAccount()

			BeforeEach(func() {
				testAcct.Nonce = 1
				testAcct.Balance = util.String("100")
				acctKey := keepers.MakeAccountKey("addr1")
				state.Set(acctKey, testAcct.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())

				testAcct2.Nonce = 20
				testAcct2.Balance = util.String("200")
				acctKey = keepers.MakeAccountKey("addr1")
				state.Set(acctKey, testAcct2.Bytes())
				_, _, err = state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should successfully return the block", func() {
				acct := ak.GetAccount("addr1", 1)
				Expect(acct).To(BeEquivalentTo(testAcct))
				acct = ak.GetAccount("addr1", 2)
				Expect(acct).To(BeEquivalentTo(testAcct2))
			})
		})
	})

	Describe(".Update", func() {
		It("should update balance", func() {
			key := util.String("addr1")
			acct := ak.GetAccount(key)
			Expect(acct.Balance).To(Equal(util.String("0")))
			Expect(acct.Nonce).To(Equal(uint64(0)))
			acct.Balance = util.String("10000")
			acct.Nonce = 2
			ak.Update(key, acct)
			acct = ak.GetAccount(key)
			Expect(acct.Balance).To(Equal(util.String("10000")))
			Expect(acct.Nonce).To(Equal(uint64(2)))
		})
	})
})
