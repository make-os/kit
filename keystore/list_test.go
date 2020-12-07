package keystore

import (
	"os"
	"path/filepath"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto"
	"github.com/make-os/kit/keystore/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("List", func() {

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

	Describe(".List", func() {
		var ks *Keystore
		When("two keys are created. one is KeyTypeUser and the other is KeyTypePush", func() {
			BeforeEach(func() {
				ks = New(keyDir)
				key := crypto.NewKeyFromIntSeed(1)
				err = ks.CreateKey(key, types.KeyTypeUser, "")
				Expect(err).To(BeNil())
				key2 := crypto.NewKeyFromIntSeed(2)
				err = ks.CreateKey(key2, types.KeyTypePush, "")
				Expect(err).To(BeNil())
			})

			It("should return 2 keys of KeyTypeUser and KeyTypePush types", func() {
				keys, err := ks.List()
				Expect(err).To(BeNil())
				Expect(keys).To(HaveLen(2))
				Expect(keys[0].GetType()).To(Or(Equal(types.KeyTypeUser), Equal(types.KeyTypePush)))
				Expect(keys[1].GetType()).To(Or(Equal(types.KeyTypeUser), Equal(types.KeyTypePush)))
			})
		})
	})
})
