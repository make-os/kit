package keepers

import (
	"os"

	"github.com/make-os/kit/crypto/ed25519"
	storagetypes "github.com/make-os/kit/storage/types"
	state2 "github.com/make-os/kit/types/state"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/pkgs/tree"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

var _ = Describe("PushKeyKeeper", func() {
	var state *tree.SafeTree
	var appDB storagetypes.Engine
	var cfg *config.AppConfig
	var err error
	var pushKeyKeeper *PushKeyKeeper

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB()
		dbTx := appDB.NewTx(true, true)
		state, err = tree.NewSafeTree(tmdb.NewMemDB(), 128)
		Expect(err).To(BeNil())
		pushKeyKeeper = NewPushKeyKeeper(state, dbTx)
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Update", func() {
		var pushKey *state2.PushKey
		BeforeEach(func() {
			pushKey = &state2.PushKey{PubKey: ed25519.StrToPublicKey("pub_key"), Address: "addr"}
			err = pushKeyKeeper.Update("pk_id", pushKey)
			Expect(err).To(BeNil())
		})

		Specify("that the key is added in the tree", func() {
			key := MakePushKeyKey("pk_id")
			_, val := state.Get(key)
			Expect(val).ToNot(BeEmpty())
			Expect(val).To(Equal(pushKey.Bytes()))
		})

		Specify("that an address->pk id index is created in the database", func() {
			key := MakeAddrPushKeyIDIndexKey("addr", "pk_id")
			_, err := appDB.Get(key)
			Expect(err).To(BeNil())
		})
	})

	Describe(".Get", func() {
		var pushKey, pushKey2 *state2.PushKey

		BeforeEach(func() {
			pushKey = &state2.PushKey{PubKey: ed25519.StrToPublicKey("pub_key"), Address: "addr"}
			err = pushKeyKeeper.Update("pk_id", pushKey)
			Expect(err).To(BeNil())
			pushKey2 = pushKeyKeeper.Get("pk_id")
		})

		Specify("that it returned the expected public key", func() {
			Expect(pushKey.Bytes()).To(Equal(pushKey2.Bytes()))
		})
	})

	Describe(".Remove", func() {
		var removed bool

		BeforeEach(func() {
			pushKey := &state2.PushKey{PubKey: ed25519.StrToPublicKey("pub_key"), Address: "addr"}
			err = pushKeyKeeper.Update("pk_id", pushKey)
			Expect(err).To(BeNil())
			Expect(pushKeyKeeper.Get("pk_id").IsNil()).To(BeFalse())
		})

		It("should remove key", func() {
			removed = pushKeyKeeper.Remove("pk_id")
			Expect(removed).To(BeTrue())
			Expect(pushKeyKeeper.Get("pk_id").IsNil()).To(BeTrue())
		})
	})

	Describe(".GetByAddress", func() {
		BeforeEach(func() {
			err = pushKeyKeeper.Update("pk_id", &state2.PushKey{PubKey: ed25519.StrToPublicKey("pub_key"), Address: "addr"})
			Expect(err).To(BeNil())
			err = pushKeyKeeper.Update("pk_id2", &state2.PushKey{PubKey: ed25519.StrToPublicKey("pub_key"), Address: "addr"})
			Expect(err).To(BeNil())
		})

		It("should return expected pk ids", func() {
			pushKeyIDs := pushKeyKeeper.GetByAddress("addr")
			Expect(pushKeyIDs).To(HaveLen(2))
			Expect(pushKeyIDs).To(ConsistOf("pk_id", "pk_id2"))
		})
	})
})
