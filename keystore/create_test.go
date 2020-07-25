package keystore

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/btcsuite/btcutil/base58"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/keystore/types"
	crypto2 "github.com/themakeos/lobe/util/crypto"
)

var _ = Describe("Create", func() {

	var err error
	path := filepath.Join("./", "test_cfg")
	keyDir := filepath.Join(path, config.KeystoreDirName)

	BeforeEach(func() {
		err = os.MkdirAll(keyDir, 0700)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(path)
		Expect(err).To(BeNil())
	})

	Describe(".Create", func() {
		When("passphrase is not provided", func() {
			BeforeEach(func() {
				ks := New(keyDir)
				key := crypto.NewKeyFromIntSeed(1)
				err = ks.CreateKey(key, types.KeyTypeAccount, "")
				Expect(err).To(BeNil())
			})

			It("should create file in the keystore directory", func() {
				entries, err := ioutil.ReadDir(keyDir)
				Expect(err).To(BeNil())
				Expect(entries).To(HaveLen(1))
			})

			Specify("that the created file has '_unprotected' in its name", func() {
				entries, _ := ioutil.ReadDir(keyDir)
				Expect(entries[0].Name()).To(ContainSubstring("_unprotected"))
			})
		})

		When("key already exist", func() {
			BeforeEach(func() {
				ks := New(keyDir)
				key := crypto.NewKeyFromIntSeed(1)
				err = ks.CreateKey(key, types.KeyTypeAccount, "")
				Expect(err).To(BeNil())
				err = ks.CreateKey(key, types.KeyTypeAccount, "")
			})

			It("should return error about an existing key", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("key already exists"))
			})
		})

		When("passphrase is provided", func() {
			var pass = "pass"

			BeforeEach(func() {
				ks := New(keyDir)
				key := crypto.NewKeyFromIntSeed(1)
				err = ks.CreateKey(key, types.KeyTypeAccount, pass)
				Expect(err).To(BeNil())
			})

			It("should create file in the key directory", func() {
				entries, err := ioutil.ReadDir(keyDir)
				Expect(err).To(BeNil())
				Expect(entries).To(HaveLen(1))
			})

			Specify("that the content of the file can be decrypted and decoded from base58check", func() {
				entries, _ := ioutil.ReadDir(keyDir)
				bz, err := ioutil.ReadFile(filepath.Join(keyDir, entries[0].Name()))
				Expect(err).To(BeNil())
				bz, err = crypto2.Decrypt(bz, hardenPassword([]byte(pass)))
				Expect(err).To(BeNil())
				_, _, err = base58.CheckDecode(string(bz))
				Expect(err).To(BeNil())
			})

			Specify("that the created file does not have '_unprotected' in its name", func() {
				entries, _ := ioutil.ReadDir(keyDir)
				Expect(entries[0].Name()).ToNot(ContainSubstring("_unprotected"))
			})
		})
	})
})
