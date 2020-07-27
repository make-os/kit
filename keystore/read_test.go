package keystore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/crypto"
	keystoretypes "github.com/themakeos/lobe/keystore/types"
	"github.com/themakeos/lobe/types"
)

var _ = Describe("Read", func() {

	var err error
	var path string
	var accountPath string
	var ks *Keystore

	BeforeEach(func() {
		path, err = ioutil.TempDir("", "")
		Expect(err).To(BeNil())
		accountPath = filepath.Join(path, "accounts")
		err := os.MkdirAll(accountPath, 0700)
		Expect(err).To(BeNil())
		Expect(err).To(BeNil())
		ks = New(accountPath)
	})

	AfterEach(func() {
		err := os.RemoveAll(path)
		Expect(err).To(BeNil())
	})

	Describe("Keystore", func() {
		Describe(".Exist", func() {
			It("should return true and err = nil when key exists", func() {
				seed := int64(1)
				address, _ := crypto.NewKey(&seed)
				passphrase := "edge123"
				err := ks.CreateKey(address, keystoretypes.KeyTypeUser, passphrase)
				Expect(err).To(BeNil())

				exist, err := ks.Exist(address.Addr().String())
				Expect(err).To(BeNil())
				Expect(exist).To(BeTrue())
			})

			It("should return false and err = nil when key does not exist", func() {
				seed := int64(1)
				address, _ := crypto.NewKey(&seed)

				exist, err := ks.Exist(address.Addr().String())
				Expect(err).To(BeNil())
				Expect(exist).To(BeFalse())
			})
		})

		Describe(".GetByIndex", func() {
			var address, address2 *crypto.Key
			BeforeEach(func() {
				seed := int64(1)
				address, _ = crypto.NewKey(&seed)
				passphrase := "edge123"
				err := ks.CreateKey(address, keystoretypes.KeyTypeUser, passphrase)
				Expect(err).To(BeNil())
				time.Sleep(1 * time.Second)

				seed = int64(2)
				address2, _ = crypto.NewKey(&seed)
				passphrase = "edge123"
				err = ks.CreateKey(address2, keystoretypes.KeyTypeUser, passphrase)
				Expect(err).To(BeNil())
			})

			It("should get accounts at index 0 and 1", func() {
				act, err := ks.GetByIndex(0)
				Expect(err).To(BeNil())
				Expect(act.GetUserAddress()).To(Equal(address.Addr().String()))
				act, err = ks.GetByIndex(1)
				Expect(act.GetUserAddress()).To(Equal(address2.Addr().String()))
			})

			It("should return err = 'key not found' when no key is found", func() {
				_, err := ks.GetByIndex(2)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(types.ErrKeyUnknown))
			})
		})

		Describe(".GetByAddress", func() {
			var address *crypto.Key

			BeforeEach(func() {
				seed := int64(1)
				address, _ = crypto.NewKey(&seed)
				passphrase := "edge123"
				err := ks.CreateKey(address, keystoretypes.KeyTypeUser, passphrase)
				Expect(err).To(BeNil())
			})

			It("should successfully get key with user address", func() {
				act, err := ks.GetByAddress(address.Addr().String())
				Expect(err).To(BeNil())
				Expect(act.GetUserAddress()).To(Equal(address.Addr().String()))
			})

			It("should successfully get key with push key address", func() {
				act, err := ks.GetByAddress(address.PushAddr().String())
				Expect(err).To(BeNil())
				Expect(act.GetPushKeyAddress()).To(Equal(address.PushAddr().String()))
			})

			It("should return err = 'key not found' when address does not exist", func() {
				_, err := ks.GetByAddress("unknown_address")
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(types.ErrKeyUnknown))
			})
		})

		Describe(".GetByIndexOrAddress", func() {
			var address *crypto.Key

			BeforeEach(func() {
				seed := int64(1)
				address, _ = crypto.NewKey(&seed)
				passphrase := "edge123"
				err := ks.CreateKey(address, keystoretypes.KeyTypeUser, passphrase)
				Expect(err).To(BeNil())
			})

			It("should return error if empty argument is provided", func() {
				_, err := ks.GetByIndexOrAddress("")
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index or address of key is required"))
			})

			It("should return error if key was not found", func() {
				_, err := ks.GetByIndexOrAddress("unknown")
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(types.ErrKeyUnknown))
			})

			It("should successfully get key by its address", func() {
				act, err := ks.GetByIndexOrAddress(address.Addr().String())
				Expect(err).To(BeNil())
				Expect(act.GetUserAddress()).To(Equal(address.Addr().String()))
			})

			It("should successfully get key by its index", func() {
				act, err := ks.GetByIndexOrAddress("0")
				Expect(err).To(BeNil())
				Expect(act.GetUserAddress()).To(Equal(address.Addr().String()))
			})
		})
	})

	Describe("StoredKey", func() {

		Describe(".Unlock", func() {
			var account keystoretypes.StoredKey
			var passphrase string

			BeforeEach(func() {
				var err error
				seed := int64(1)

				address, _ := crypto.NewKey(&seed)
				passphrase = "edge123"
				err = ks.CreateKey(address, keystoretypes.KeyTypeUser, passphrase)
				Expect(err).To(BeNil())

				accounts, err := ks.List()
				Expect(err).To(BeNil())
				Expect(accounts).To(HaveLen(1))
				account = accounts[0]
			})

			It("should return err = 'invalid passphrase' when passphrase is invalid", func() {
				err := account.Unlock("invalid")
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(types.ErrInvalidPassphrase))
			})

			It("should return nil when decryption is successful", func() {
				err := account.Unlock(passphrase)
				Expect(err).To(BeNil())
				Expect(account.GetKey()).ToNot(BeNil())
			})
		})
	})

	Describe("StoredKeyMeta", func() {
		Describe(".HasKey", func() {
			It("should return false when key does not exist", func() {
				sa := StoredKey{meta: map[string]interface{}{}}
				r := sa.meta.HasKey("key")
				Expect(r).To(BeFalse())
			})

			It("should return true when key exist", func() {
				sa := StoredKey{meta: map[string]interface{}{"key": 2}}
				r := sa.meta.HasKey("key")
				Expect(r).To(BeTrue())
			})
		})

		Describe(".Get", func() {
			It("should return nil when key does not exist", func() {
				sa := StoredKey{meta: map[string]interface{}{}}
				r := sa.meta.Get("key")
				Expect(r).To(BeNil())
			})

			It("should return expected value when key exist", func() {
				sa := StoredKey{meta: map[string]interface{}{"key": 2}}
				r := sa.meta.Get("key")
				Expect(r).To(Equal(2))
			})
		})
	})

})
