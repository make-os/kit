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
		stakes = AccountStakes(map[string]*StakeInfo{"s1": &StakeInfo{Value: util.String("10")}})
		acct = &Account{Balance: util.String("10"), Nonce: 2, Stakes: stakes}
		acctBs = []uint8{
			148, 162, 49, 48, 207, 0, 0, 0, 0, 0, 0, 0, 2, 129, 162, 115, 49, 130, 165, 86, 97,
			108, 117, 101, 162, 49, 48, 172, 85, 110, 98, 111, 110, 100, 72, 101, 105, 103, 104,
			116, 207, 0, 0, 0, 0, 0, 0, 0, 0, 203, 0, 0, 0, 0, 0, 0, 0, 0,
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
		When("account has a stake entry of value=10 and unbond height=1", func() {
			BeforeEach(func() {
				acct.Stakes = AccountStakes(map[string]*StakeInfo{"s1": &StakeInfo{
					Value:        util.String("10"),
					UnbondHeight: 1,
				}})
			})

			It("should return balance=10 when chain height = 1", func() {
				Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("10")))
			})
		})

		When("account has a stake entry of value=10 and unbond height=100", func() {
			BeforeEach(func() {
				acct.Stakes = AccountStakes(map[string]*StakeInfo{"s1": &StakeInfo{
					Value:        util.String("10"),
					UnbondHeight: 100,
				}})
			})

			It("should return balance=0 when chain height = 1", func() {
				Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("0")))
			})
		})

		When("account has no stake entry", func() {
			BeforeEach(func() {
				acct.Stakes = BareAccountStakes()
			})

			It("should return balance=10 at any chain height", func() {
				Expect(acct.GetSpendableBalance(1)).To(Equal(util.String("10")))
				Expect(acct.GetSpendableBalance(1000)).To(Equal(util.String("10")))
			})
		})
	})

	Describe(".CleanUnbonded", func() {
		When("account's unbond height is 1000", func() {
			var stake *StakeInfo
			BeforeEach(func() {
				stake = &StakeInfo{Value: util.String("10"), UnbondHeight: 1000}
				stakes = AccountStakes(map[string]*StakeInfo{"s1": stake})
				acct = &Account{Balance: util.String("10"), Nonce: 2, Stakes: stakes}
			})

			When("unbondHeight arg is 500", func() {
				It("should not remove the stake entry", func() {
					acct.CleanUnbonded(500)
					Expect(acct.Stakes).To(HaveLen(1))
					Expect(acct.Stakes.Get("s1")).To(Equal(stake))
				})
			})
		})

		When("account's unbond height is 0", func() {
			var stake *StakeInfo
			BeforeEach(func() {
				stake = &StakeInfo{Value: util.String("10"), UnbondHeight: 0}
				stakes = AccountStakes(map[string]*StakeInfo{"s1": stake})
				acct = &Account{Balance: util.String("10"), Nonce: 2, Stakes: stakes}
			})

			It("should not remove the stake entry", func() {
				acct.CleanUnbonded(500)
				Expect(acct.Stakes).To(HaveLen(1))
				Expect(acct.Stakes.Get("s1")).To(Equal(stake))
			})
		})

		When("account's unbond height is 1000", func() {
			var stake *StakeInfo
			BeforeEach(func() {
				stake = &StakeInfo{Value: util.String("10"), UnbondHeight: 1000}
				stakes = AccountStakes(map[string]*StakeInfo{"s1": stake})
				acct = &Account{Balance: util.String("10"), Nonce: 2, Stakes: stakes}
			})

			When("unbondHeight arg is 1000", func() {
				It("should remove the stake entry", func() {
					acct.CleanUnbonded(1000)
					Expect(acct.Stakes).To(HaveLen(0))
					Expect(acct.Stakes.Get("s1")).To(Equal(BareStakeInfo()))
				})
			})
		})
	})
})

var _ = Describe("AccountStakes", func() {
	Describe(".Add", func() {
		It("should add two stake balances", func() {
			stakes := AccountStakes(map[string]*StakeInfo{})
			stakes.Add(StakeTypeValidator, util.String("10"), 0)
			stakes.Add(StakeTypeValidator, util.String("13"), 0)
			Expect(len(stakes)).To(Equal(2))
		})
	})

	Describe(".Has", func() {
		It("should return true when stake exist", func() {
			stakes := AccountStakes(map[string]*StakeInfo{})
			key := stakes.Add(StakeTypeValidator, util.String("10"), 1)
			Expect(stakes.Has(key)).To(BeTrue())
		})

		It("should return false when stake does not exist", func() {
			stakes := AccountStakes(map[string]*StakeInfo{})
			stakes.Add(StakeTypeValidator, util.String("10"), 1)
			Expect(stakes.Has("s2")).To(BeFalse())
		})
	})

	Describe(".TotalStaked", func() {
		When("current chain height is 100 and stake unbond height is below 100", func() {
			It("should return total stakes of 0 since the stake is unbonded", func() {
				stakes := AccountStakes(map[string]*StakeInfo{})
				stakes.Add(StakeTypeValidator, util.String("10"), 20)
				totalStaked := stakes.TotalStaked(100)
				Expect(totalStaked).To(Equal(util.String("0")))
			})
		})

		When("current chain height is anything and stake unbond height is 0", func() {
			It("should return total stakes of 10 since the stake is forever bonded", func() {
				stakes := AccountStakes(map[string]*StakeInfo{})
				stakes.Add(StakeTypeValidator, util.String("10"), 0)
				totalStaked := stakes.TotalStaked(100)
				Expect(totalStaked).To(Equal(util.String("10")))
			})
		})

		When("current chain height is 100 and stake unbond height is above 100", func() {
			It("should return total stakes of 0 since the stake is unbonded", func() {
				stakes := AccountStakes(map[string]*StakeInfo{})
				stakes.Add(StakeTypeValidator, util.String("10"), 100)
				totalStaked := stakes.TotalStaked(100)
				Expect(totalStaked).To(Equal(util.String("0")))
			})
		})

		When("current chain height is 100 and stake unbond height is above 101", func() {
			It("should return total stakes of 10 since the stake is bonded", func() {
				stakes := AccountStakes(map[string]*StakeInfo{})
				stakes.Add(StakeTypeValidator, util.String("10"), 101)
				totalStaked := stakes.TotalStaked(100)
				Expect(totalStaked).To(Equal(util.String("10")))
			})
		})
	})

	Describe(".Get", func() {
		It("should return zero ('0') when stake is not found", func() {
			stakes := AccountStakes(map[string]*StakeInfo{})
			Expect(stakes.Get("unknown")).To(Equal(&StakeInfo{Value: util.String("0")}))
		})

		It("should return expected value when stake is found", func() {
			stakes := AccountStakes(map[string]*StakeInfo{})
			key := stakes.Add(StakeTypeValidator, util.String("10"), 0)
			Expect(stakes.Get(key).Value).To(Equal(util.String("10")))
		})
	})

	Describe(".Remove", func() {
		Context("with existing entry of value=10 and key=v0 and unbondHeight=0", func() {
			It("should remove entry with stakeType=v, value=10, unbondHeight=0", func() {
				stakes := AccountStakes(map[string]*StakeInfo{})
				key := stakes.Add("v", util.String("10"), 0)
				Expect(key).To(Equal("v0"))
				Expect(stakes).To(HaveLen(1))
				stakes.Remove("v", util.String("10"), 0)
				Expect(stakes).To(HaveLen(0))
			})
		})

		Context("with existing entry of value=10 and key=v0 and unbondHeight=1", func() {
			It("should not remove entry with stakeType=v, value=10, unbondHeight=0", func() {
				stakes := AccountStakes(map[string]*StakeInfo{})
				key := stakes.Add("v", util.String("10"), 1)
				Expect(key).To(Equal("v0"))
				Expect(stakes).To(HaveLen(1))
				stakes.Remove("v", util.String("10"), 0)
				Expect(stakes).To(HaveLen(1))
			})
		})

		Context("with existing entry of value=10.5 and key=v0 and unbondHeight=0", func() {
			It("should not remove entry with stakeType=v, value=10, unbondHeight=0", func() {
				stakes := AccountStakes(map[string]*StakeInfo{})
				key := stakes.Add("v", util.String("10.5"), 0)
				Expect(key).To(Equal("v0"))
				Expect(stakes).To(HaveLen(1))
				stakes.Remove("v", util.String("10"), 0)
				Expect(stakes).To(HaveLen(1))
			})
		})

		Context("with existing entry of value=10 and key=s0 and unbondHeight=0", func() {
			It("should not remove entry with stakeType=v, value=10, unbondHeight=0", func() {
				stakes := AccountStakes(map[string]*StakeInfo{})
				key := stakes.Add("s", util.String("10"), 0)
				Expect(key).To(Equal("s0"))
				Expect(stakes).To(HaveLen(1))
				stakes.Remove("v", util.String("10"), 0)
				Expect(stakes).To(HaveLen(1))
			})
		})

		Context("with 2 existing entry of value=10 and key=v0 and unbondHeight=0 | value=10 and key=v1 and unbondHeight=0", func() {
			var stakes AccountStakes
			BeforeEach(func() {
				stakes = AccountStakes(map[string]*StakeInfo{})
				key := stakes.Add("v", util.String("10"), 0)
				Expect(key).To(Equal("v0"))
				key2 := stakes.Add("v", util.String("10"), 0)
				Expect(key2).To(Equal("v1"))
				Expect(stakes).To(HaveLen(2))
			})

			It("should remove entry with stakeType=v, value=10, unbondHeight=0 and leave one entry", func() {
				rmKey := stakes.Remove("v", util.String("10"), 0)
				Expect(rmKey).To(Or(Equal("v0"), Equal("v1")))
				Expect(stakes).To(HaveLen(1))
			})
		})
	})

	Describe(".UpdateUnbondHeight", func() {
		Context("with existing entry of value=10 and key=v0 and unbondHeight=0", func() {
			It("should find entry with stakeType=v, value=10, unbondHeight=0 and unbondHeight=10", func() {
				stakes := AccountStakes(map[string]*StakeInfo{})
				key := stakes.Add("v", util.String("10"), 0)
				Expect(key).To(Equal("v0"))
				Expect(stakes).To(HaveLen(1))
				key2 := stakes.UpdateUnbondHeight("v", util.String("10"), 0, 10)
				Expect(key).To(Equal(key2))
				stake := stakes[key2]
				Expect(stake.UnbondHeight).To(Equal(uint64(10)))
			})
		})
	})
})
