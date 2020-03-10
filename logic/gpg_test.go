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
				gpgKey := logic.gpgPubKeyKeeper.GetGPGPubKey(gpgID, 0)
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
})
