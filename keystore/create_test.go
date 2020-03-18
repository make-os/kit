package keystore

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/btcsuite/btcutil/base58"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
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
				err = ks.CreateKey(key, core.KeyTypeAccount, "")
				Expect(err).To(BeNil())
			})

			It("should create file in the privKey directory", func() {
				entries, err := ioutil.ReadDir(keyDir)
				Expect(err).To(BeNil())
				Expect(entries).To(HaveLen(1))
			})

			Specify("that the created file has '_unsafe' in its name", func() {
				entries, _ := ioutil.ReadDir(keyDir)
				Expect(entries[0].Name()).To(ContainSubstring("_unsafe"))
			})
		})

		When("passphrase is provided", func() {
			var pass = "pass"

			BeforeEach(func() {
				ks := New(keyDir)
				key := crypto.NewKeyFromIntSeed(1)
				err = ks.CreateKey(key, core.KeyTypeAccount, pass)
				Expect(err).To(BeNil())
			})

			It("should create file in the privKey directory", func() {
				entries, err := ioutil.ReadDir(keyDir)
				Expect(err).To(BeNil())
				Expect(entries).To(HaveLen(1))
			})

			Specify("that the content of the file can be decrypted and decoded from base58check", func() {
				entries, _ := ioutil.ReadDir(keyDir)
				bz, err := ioutil.ReadFile(filepath.Join(keyDir, entries[0].Name()))
				Expect(err).To(BeNil())
				bz, err = util.Decrypt(bz, hardenPassword([]byte(pass)))
				Expect(err).To(BeNil())
				_, _, err = base58.CheckDecode(string(bz))
				Expect(err).To(BeNil())
			})

			Specify("that the created file does not have '_unsafe' in its name", func() {
				entries, _ := ioutil.ReadDir(keyDir)
				Expect(entries[0].Name()).ToNot(ContainSubstring("_unsafe"))
			})
		})
	})
})
