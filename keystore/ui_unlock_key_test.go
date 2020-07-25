package keystore

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	types2 "github.com/themakeos/lobe/keystore/types"
	"github.com/themakeos/lobe/types"
)

var _ = Describe("Reveal", func() {
	var err error
	var oldStdout = os.Stdout
	path := filepath.Join("./", "test_cfg")
	keyDir := filepath.Join(path, config.KeystoreDirName)

	BeforeEach(func() {
		err = os.MkdirAll(keyDir, 0700)
		Expect(err).To(BeNil())
		_, w, _ := os.Pipe()
		os.Stdout = w
	})

	AfterEach(func() {
		os.Stdout = oldStdout
		err = os.RemoveAll(path)
		Expect(err).To(BeNil())
	})

	Describe(".UIUnlockKey", func() {
		var ks *Keystore
		BeforeEach(func() {
			ks = New(keyDir)
		})

		When("address does not exist", func() {
			It("should return err", func() {
				_, err := ks.UIUnlockKey("unknown", "pass", "")
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(types.ErrKeyUnknown))
			})
		})

		When("key has no passphrase (unprotected)", func() {
			var key *crypto.Key

			BeforeEach(func() {
				key = crypto.NewKeyFromIntSeed(1)
				err := ks.CreateKey(key, types2.KeyTypeAccount, "")
				Expect(err).To(BeNil())
			})

			It("should unlock account without providing a passphrase", func() {
				acct, err := ks.UIUnlockKey(key.Addr().String(), "", "")
				Expect(err).To(BeNil())
				Expect(acct).ToNot(BeNil())
				Expect(acct.GetAddress()).To(Equal(key.Addr().String()))
			})
		})

		When("key has passphrase (safe)", func() {
			var key *crypto.Key

			BeforeEach(func() {
				key = crypto.NewKeyFromIntSeed(1)
				err := ks.CreateKey(key, types2.KeyTypeAccount, "my_pass")
				Expect(err).To(BeNil())
			})

			It("should prompt user for passphrase to unlock account", func() {
				var prompted bool
				ks.getPassword = func(s string, i ...interface{}) string {
					prompted = true
					return "my_pass"
				}
				acct, err := ks.UIUnlockKey(key.Addr().String(), "", "")
				Expect(err).To(BeNil())
				Expect(acct).ToNot(BeNil())
				Expect(acct.GetAddress()).To(Equal(key.Addr().String()))
				Expect(prompted).To(BeTrue())
			})

			It("should return error when user passphrase is incorrect", func() {
				var prompted bool
				ks.getPassword = func(s string, i ...interface{}) string {
					prompted = true
					return "my_wrong_pass"
				}
				_, err := ks.UIUnlockKey(key.Addr().String(), "", "")
				Expect(prompted).To(BeTrue())
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("invalid passphrase"))
			})
		})
	})
})
