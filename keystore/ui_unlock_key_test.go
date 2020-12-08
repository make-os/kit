package keystore

import (
	"os"
	"path/filepath"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	types2 "github.com/make-os/kit/keystore/types"
	"github.com/make-os/kit/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UnlockKeyUI", func() {
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

	Describe(".UnlockKeyUI", func() {
		var ks *Keystore
		BeforeEach(func() {
			ks = New(keyDir)
		})

		When("address does not exist", func() {
			It("should return err", func() {
				_, _, err := ks.UnlockKeyUI("unknown", "pass", "")
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(types.ErrKeyUnknown))
			})
		})

		When("key has no passphrase (unprotected)", func() {
			var key *ed25519.Key

			BeforeEach(func() {
				key = ed25519.NewKeyFromIntSeed(1)
				err := ks.CreateKey(key, types2.KeyTypeUser, "")
				Expect(err).To(BeNil())
			})

			It("should unlock account without providing a passphrase", func() {
				acct, _, err := ks.UnlockKeyUI(key.Addr().String(), "", "")
				Expect(err).To(BeNil())
				Expect(acct).ToNot(BeNil())
				Expect(acct.GetUserAddress()).To(Equal(key.Addr().String()))
			})
		})

		When("key has passphrase", func() {
			var key *ed25519.Key

			BeforeEach(func() {
				key = ed25519.NewKeyFromIntSeed(1)
				err := ks.CreateKey(key, types2.KeyTypeUser, "my_pass")
				Expect(err).To(BeNil())
			})

			It("should prompt user for passphrase to unlock account", func() {
				var prompted bool
				ks.getPassword = func(s string, i ...interface{}) (string, error) {
					prompted = true
					return "my_pass", nil
				}
				acct, passphrase, err := ks.UnlockKeyUI(key.Addr().String(), "", "")
				Expect(err).To(BeNil())
				Expect(acct).ToNot(BeNil())
				Expect(acct.GetUserAddress()).To(Equal(key.Addr().String()))
				Expect(prompted).To(BeTrue())
				Expect(passphrase).To(Equal("my_pass"))
			})

			It("should return error when user passphrase is incorrect", func() {
				var prompted bool
				ks.getPassword = func(s string, i ...interface{}) (string, error) {
					prompted = true
					return "my_wrong_pass", nil
				}
				_, _, err := ks.UnlockKeyUI(key.Addr().String(), "", "")
				Expect(prompted).To(BeTrue())
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("invalid passphrase"))
			})
		})
	})
})
