package logic

import (
	"crypto/rsa"
	"gitlab.com/makeos/mosdef/types/state"
	"io/ioutil"
	"os"

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

	Describe(".execAddGPGKey", func() {
		var err error
		var sender = crypto.NewKeyFromIntSeed(1)

		BeforeEach(func() {
			logic.AccountKeeper().Update(sender.Addr(), &state.Account{
				Balance:             util.String("10"),
				Stakes:              state.BareAccountStakes(),
				DelegatorCommission: 10,
			})
		})

		When("successful", func() {
			var gpgPubKey string

			BeforeEach(func() {
				gpgPubKey = string(getTestFile("gpgpubkey.pub"))
				senderPubKey := sender.PubKey().MustBytes32()
				err = txLogic.execAddGPGKey(senderPubKey, gpgPubKey, "1.5", 0)
				Expect(err).To(BeNil())
			})

			Specify("that the gpg public key was added to the tree", func() {
				entity, _ := crypto.PGPEntityFromPubKey(gpgPubKey)
				gpgID := util.RSAPubKeyID(entity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				gpgKey := logic.gpgPubKeyKeeper.GetGPGPubKey(gpgID, 0)
				Expect(gpgKey.IsNil()).To(BeFalse())
				Expect(gpgKey.Address).To(Equal(sender.Addr()))
				Expect(gpgKey.PubKey).To(Equal(gpgPubKey))
			})

			Specify("that fee is deducted from sender account", func() {
				acct := logic.AccountKeeper().GetAccount(sender.Addr())
				Expect(acct.GetBalance()).To(Equal(util.String("8.5")))
			})
		})
	})
})
