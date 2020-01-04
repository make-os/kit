package keepers

import (
	"os"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

var _ = Describe("GPGKeeper", func() {
	var state *tree.SafeTree
	var appDB storage.Engine
	var cfg *config.AppConfig
	var err error
	var gpgKeeper *GPGPubKeyKeeper

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB(cfg)
		dbTx := appDB.NewTx(true, true)
		state = tree.NewSafeTree(tmdb.NewMemDB(), 128)
		gpgKeeper = NewGPGPubKeyKeeper(state, dbTx)
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Update", func() {
		var gpgPK *types.GPGPubKey
		BeforeEach(func() {
			gpgPK = &types.GPGPubKey{PubKey: "pub_key", Address: "addr"}
			err = gpgKeeper.Update("pk_id", gpgPK)
			Expect(err).To(BeNil())
		})

		Specify("that the key is added in the tree", func() {
			key := MakeGPGPubKeyKey("pk_id")
			_, val := state.Get(key)
			Expect(val).ToNot(BeEmpty())
			Expect(val).To(Equal(gpgPK.Bytes()))
		})

		Specify("that an address->pk id index is created in the database", func() {
			key := MakeAddrGPGPkIDIndexKey("addr", "pk_id")
			_, err := appDB.Get(key)
			Expect(err).To(BeNil())
		})
	})

	Describe(".GetGPGPubKey", func() {
		var gpgPK, gpgPK2 *types.GPGPubKey

		BeforeEach(func() {
			gpgPK = &types.GPGPubKey{PubKey: "pub_key", Address: "addr"}
			err = gpgKeeper.Update("pk_id", gpgPK)
			Expect(err).To(BeNil())
			gpgPK2 = gpgKeeper.GetGPGPubKey("pk_id")
		})

		Specify("that it returned the expected public key", func() {
			Expect(gpgPK).To(Equal(gpgPK2))
		})
	})

	Describe(".GetPubKeyIDs", func() {
		BeforeEach(func() {
			err = gpgKeeper.Update("pk_id", &types.GPGPubKey{PubKey: "pub_key", Address: "addr"})
			Expect(err).To(BeNil())
			err = gpgKeeper.Update("pk_id2", &types.GPGPubKey{PubKey: "pub_key", Address: "addr"})
			Expect(err).To(BeNil())
		})

		It("should return expected pk ids", func() {
			pkIDs := gpgKeeper.GetPubKeyIDs("addr")
			Expect(pkIDs).To(HaveLen(2))
			Expect(pkIDs).To(ConsistOf("pk_id", "pk_id2"))
		})
	})
})
