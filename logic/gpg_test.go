package logic

import (
	"crypto/rsa"
	"io/ioutil"
	"os"

	"gitlab.com/makeos/mosdef/types/state"

	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
)

func getTestFile(filename string) []byte {
	bz, err := ioutil.ReadFile("./testdata/" + filename)
	if err != nil {
		panic(err)
	}
	return bz
}

var _ = Describe("GPG", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *Logic
	var txLogic *Transaction
	var ctrl *gomock.Controller

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)
		txLogic = &Transaction{logic: logic}
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".execRegisterGPGKey", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             "10",
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 10,
			})
		})

		When("successful", func() {
			var gpgPubKey string
			var scopes = []string{"repo1", "repo2"}
			var feeCap = util.String("10")

			BeforeEach(func() {
				gpgPubKey = string(getTestFile("gpgpubkey.pub"))
				senderPubKey := sender.PubKey().MustBytes32()
				err = txLogic.execRegisterGPGKey(senderPubKey, gpgPubKey, scopes, feeCap, "1.5", 0)
				Expect(err).To(BeNil())
			})

			Specify("that the gpg public key was added to the tree", func() {
				entity, _ := crypto.PGPEntityFromPubKey(gpgPubKey)
				gpgID := util.CreateGPGIDFromRSA(entity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				gpgKey := logic.gpgPubKeyKeeper.Get(gpgID, 0)
				Expect(gpgKey.IsNil()).To(BeFalse())
				Expect(gpgKey.Address).To(Equal(sender.Addr()))
				Expect(gpgKey.PubKey).To(Equal(gpgPubKey))
				Expect(gpgKey.Scopes).To(Equal(scopes))
				Expect(gpgKey.FeeCap).To(Equal(feeCap))
			})

			Specify("that fee is deducted from sender account", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("8.5")))
			})
		})
	})

	Describe(".execUpDelGPGKey", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             "10",
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 10,
			})
		})

		When("delete is set to true", func() {
			var gpgKeyID = "gpg1_abc"
			BeforeEach(func() {
				key := state.BareGPGPubKey()
				key.Address = "addr1"
				logic.GPGPubKeyKeeper().Update(gpgKeyID, key)
				Expect(logic.GPGPubKeyKeeper().Get(gpgKeyID).IsNil()).To(BeFalse())

				senderPubKey := sender.PubKey().MustBytes32()
				err = txLogic.execUpDelGPGKey(senderPubKey, gpgKeyID, nil,
					nil, true, "", "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should delete key", func() {
				Expect(logic.GPGPubKeyKeeper().Get(gpgKeyID).IsNil()).To(BeTrue())
			})

			Specify("that fee is deducted from sender account", func() {
				acct := logic.AccountKeeper().Get(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("8.5")))
			})
		})

		When("removeScope includes indices 0,2", func() {
			var gpgKeyID = "gpg1_abc"
			BeforeEach(func() {
				key := state.BareGPGPubKey()
				key.Address = "addr1"
				key.Scopes = []string{"scope1", "scope2", "scope3"}
				logic.GPGPubKeyKeeper().Update(gpgKeyID, key)
				Expect(logic.GPGPubKeyKeeper().Get(gpgKeyID).IsNil()).To(BeFalse())

				senderPubKey := sender.PubKey().MustBytes32()
				rmScopes := []int{0, 2}
				err = txLogic.execUpDelGPGKey(senderPubKey, gpgKeyID, nil,
					rmScopes, false, "", "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should remove scopes at indices 0,2", func() {
				key := logic.GPGPubKeyKeeper().Get(gpgKeyID)
				Expect(key.Scopes).To(HaveLen(1))
				Expect(key.Scopes).To(ContainElement("scope2"))
			})
		})

		When("removeScope includes indices 0,5,2 or 0,2,5", func() {
			var gpgKeyID = "gpg1_abc"
			for _, indicesSlice := range [][]int{{0, 5, 2}, {0, 2, 5}} {
				BeforeEach(func() {
					key := state.BareGPGPubKey()
					key.Address = "addr1"
					key.Scopes = []string{"scope1", "scope2", "scope3", "scope4", "scope5", "scope6", "scope7"}
					logic.GPGPubKeyKeeper().Update(gpgKeyID, key)
					Expect(logic.GPGPubKeyKeeper().Get(gpgKeyID).IsNil()).To(BeFalse())

					senderPubKey := sender.PubKey().MustBytes32()
					rmScopes := indicesSlice
					err = txLogic.execUpDelGPGKey(senderPubKey, gpgKeyID, nil,
						rmScopes, false, "", "1.5", 0)
					Expect(err).To(BeNil())
				})

				It("should remove scopes at indices 0,2", func() {
					key := logic.GPGPubKeyKeeper().Get(gpgKeyID)
					Expect(key.Scopes).To(HaveLen(4))
					Expect(key.Scopes).ToNot(ContainElement("scope1"))
					Expect(key.Scopes).ToNot(ContainElement("scope3"))
					Expect(key.Scopes).ToNot(ContainElement("scope6"))
				})
			}
		})

		When("addScopes includes scope10, scope11", func() {
			var gpgKeyID = "gpg1_abc"
			BeforeEach(func() {
				key := state.BareGPGPubKey()
				key.Address = "addr1"
				key.Scopes = []string{"scope1", "scope2", "scope3"}
				logic.GPGPubKeyKeeper().Update(gpgKeyID, key)
				Expect(logic.GPGPubKeyKeeper().Get(gpgKeyID).IsNil()).To(BeFalse())

				senderPubKey := sender.PubKey().MustBytes32()
				addScopes := []string{"scope10", "scope11"}
				err = txLogic.execUpDelGPGKey(senderPubKey, gpgKeyID, addScopes,
					nil, false, "", "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should add scopes scope10, scope11", func() {
				key := logic.GPGPubKeyKeeper().Get(gpgKeyID)
				Expect(key.Scopes).To(HaveLen(5))
				Expect(key.Scopes).To(ContainElement("scope10"))
				Expect(key.Scopes).To(ContainElement("scope11"))
			})
		})

		When("feeCap is set", func() {
			var gpgKeyID = "gpg1_abc"
			BeforeEach(func() {
				key := state.BareGPGPubKey()
				key.Address = "addr1"
				logic.GPGPubKeyKeeper().Update(gpgKeyID, key)
				Expect(logic.GPGPubKeyKeeper().Get(gpgKeyID).IsNil()).To(BeFalse())

				senderPubKey := sender.PubKey().MustBytes32()
				err = txLogic.execUpDelGPGKey(senderPubKey, gpgKeyID, nil,
					nil, false, "100", "1.5", 0)
				Expect(err).To(BeNil())
			})

			It("should update fee cap", func() {
				key := logic.GPGPubKeyKeeper().Get(gpgKeyID)
				Expect(key.FeeCap).To(Equal(util.String("100")))
			})
		})
	})
})
