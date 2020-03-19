package keystore

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
)

var _ = Describe("Read", func() {

	path := filepath.Join("./", "test_cfg")
	accountPath := filepath.Join(path, "accounts")

	BeforeEach(func() {
		err := os.MkdirAll(accountPath, 0700)
		Expect(err).To(BeNil())
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err := os.RemoveAll(path)
		Expect(err).To(BeNil())
	})

	Describe("Keystore", func() {
		Describe(".Exist", func() {
			ks := New(accountPath)
			It("should return true and err = nil when key exists", func() {
				seed := int64(1)
				address, _ := crypto.NewKey(&seed)
				passphrase := "edge123"
				err := ks.CreateKey(address, core.KeyTypeAccount, passphrase)
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
			am := New(accountPath)

			BeforeEach(func() {
				seed := int64(1)
				address, _ = crypto.NewKey(&seed)
				passphrase := "edge123"
				err := am.CreateKey(address, core.KeyTypeAccount, passphrase)
				Expect(err).To(BeNil())
				time.Sleep(1 * time.Second)

				seed = int64(2)
				address2, _ = crypto.NewKey(&seed)
				passphrase = "edge123"
				err = am.CreateKey(address2, core.KeyTypeAccount, passphrase)
				Expect(err).To(BeNil())
			})

			It("should get accounts at index 0 and 1", func() {
				act, err := am.GetByIndex(0)
				Expect(err).To(BeNil())
				Expect(act.GetAddress()).To(Equal(address.Addr().String()))
				act, err = am.GetByIndex(1)
				Expect(act.GetAddress()).To(Equal(address2.Addr().String()))
			})

			It("should return err = 'key not found' when no key is found", func() {
				_, err := am.GetByIndex(2)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(types.ErrKeyUnknown))
			})
		})

		Describe(".GetByAddress", func() {

			var address *crypto.Key
			am := New(accountPath)

			BeforeEach(func() {
				seed := int64(1)
				address, _ = crypto.NewKey(&seed)
				passphrase := "edge123"
				err := am.CreateKey(address, core.KeyTypeAccount, passphrase)
				Expect(err).To(BeNil())
			})

			It("should successfully get key with address", func() {
				act, err := am.GetByAddress(address.Addr().String())
				Expect(err).To(BeNil())
				Expect(act.GetAddress()).To(Equal(address.Addr().String()))
			})

			It("should return err = 'key not found' when address does not exist", func() {
				_, err := am.GetByAddress("unknown_address")
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(types.ErrKeyUnknown))
			})
		})

		Describe(".GetByIndexOrAddress", func() {

			var address *crypto.Key
			am := New(accountPath)

			BeforeEach(func() {
				seed := int64(1)
				address, _ = crypto.NewKey(&seed)
				passphrase := "edge123"
				err := am.CreateKey(address, core.KeyTypeAccount, passphrase)
				Expect(err).To(BeNil())
			})

			It("should successfully get key by its address", func() {
				act, err := am.GetByIndexOrAddress(address.Addr().String())
				Expect(err).To(BeNil())
				Expect(act.GetAddress()).To(Equal(address.Addr().String()))
			})

			It("should successfully get key by its index", func() {
				act, err := am.GetByIndexOrAddress("0")
				Expect(err).To(BeNil())
				Expect(act.GetAddress()).To(Equal(address.Addr().String()))
			})
		})
	})

	Describe("StoredKey", func() {

		Describe(".Unlock", func() {
			var account core.StoredKey
			var passphrase string
			am := New(accountPath)

			BeforeEach(func() {
				var err error
				seed := int64(1)

				address, _ := crypto.NewKey(&seed)
				passphrase = "edge123"
				err = am.CreateKey(address, core.KeyTypeAccount, passphrase)
				Expect(err).To(BeNil())

				accounts, err := am.List()
				Expect(err).To(BeNil())
				Expect(accounts).To(HaveLen(1))
				account = accounts[0]
			})

			It("should return err = 'invalid passphrase' when passphrase is invalid", func() {
				err := account.Unlock("invalid")
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(types.ErrInvalidPassprase))
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
