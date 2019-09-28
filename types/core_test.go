package types

import (
	"github.com/makeos/mosdef/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Account", func() {
	var stakes AccountStakes
	var acct *Account
	var acctBs []byte

	BeforeEach(func() {
		stakes = AccountStakes(map[string]util.String{"s1": util.String("10")})
		acct = &Account{Balance: util.String("10"), Nonce: 2, Stakes: stakes}
		acctBs = []uint8{
			147, 162, 49, 48, 207, 0, 0, 0, 0, 0, 0, 0, 2, 129, 162, 115, 49, 162, 49, 48,
		}
	})

	Describe(".Bytes", func() {
		It("should return serialized byte", func() {
			bs := acct.Bytes()
			Expect(bs).To(Equal(acctBs))
		})
	})

	Describe(".NewAccountFromBytes", func() {
		It("should return expected account", func() {
			res, err := NewAccountFromBytes(acctBs)
			Expect(err).To(BeNil())
			Expect(res).To(BeEquivalentTo(acct))
		})
	})

	Describe(".GetBalance", func() {
		It("should return expected balance", func() {
			Expect(acct.GetBalance()).To(Equal(acct.Balance))
		})
	})

	Describe(".GetSpendableBalance", func() {
		When("account has no staked balance", func() {
			It("should return expected balance", func() {
				Expect(acct.GetSpendableBalance()).To(Equal(util.String("0")))
			})
		})

		When("account has a staked balance", func() {
			It("should return expected balance", func() {
				acct.Stakes = BareAccount().Stakes
				acct.Stakes.Add("s1", util.String("2"))
				acct.Stakes.Add("s2", util.String("1"))
				Expect(acct.GetSpendableBalance()).To(Equal(util.String("7")))
			})
		})
	})
})

var _ = Describe("AccountStakes", func() {
	Describe(".Add", func() {
		It("should add two stake balances", func() {
			stakes := AccountStakes(map[string]util.String{})
			stakes.Add("s1", util.String("10"))
			stakes.Add("s2", util.String("13"))
			Expect(len(stakes)).To(Equal(2))
		})
	})

	Describe(".Has", func() {
		It("should return true when stake exist", func() {
			stakes := AccountStakes(map[string]util.String{})
			stakes.Add("s1", util.String("10"))
			Expect(stakes.Has("s1")).To(BeTrue())
		})

		It("should return false when stake does not exist", func() {
			stakes := AccountStakes(map[string]util.String{})
			stakes.Add("s1", util.String("10"))
			Expect(stakes.Has("s2")).To(BeFalse())
		})
	})

	Describe(".TotalStaked", func() {
		It("should return expected total stakes", func() {
			stakes := AccountStakes(map[string]util.String{})
			stakes.Add("s1", util.String("10"))
			stakes.Add("s2", util.String("13"))
			totalStaked := stakes.TotalStaked()
			Expect(totalStaked).To(Equal(util.String("23")))
		})
	})

	Describe(".Get", func() {
		It("should return zero ('0') when stake is not found", func() {
			stakes := AccountStakes(map[string]util.String{})
			Expect(stakes.Get("unknown")).To(Equal(util.String("0")))
		})

		It("should return expected value when stake is found", func() {
			stakes := AccountStakes(map[string]util.String{})
			stakes.Add("s", util.String("10"))
			Expect(stakes.Get("s")).To(Equal(util.String("10")))
		})
	})
})
