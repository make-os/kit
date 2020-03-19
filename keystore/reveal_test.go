package keystore

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Reveal", func() {
	var err error
	var cfg *config.AppConfig

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".readPassFromFile", func() {
		When("path to file is unknown", func() {
			It("should return error", func() {
				_, err := readPassFromFile("unknown/file.txt")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to read passphrase file"))
			})
		})

		When("path to file is actually a directory", func() {
			It("should return error", func() {
				dirPath := filepath.Join(cfg.DataDir(), util.RandString(5))
				os.MkdirAll(dirPath, 0700)
				_, err := readPassFromFile(dirPath)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to read passphrase file"))
			})
		})

		When("path is a file that contains a text", func() {
			It("should return error", func() {
				filePath := filepath.Join(cfg.DataDir(), util.RandString(5))
				err = ioutil.WriteFile(filePath, []byte("passphrase"), 0644)
				Expect(err).To(BeNil())
				pass, err := readPassFromFile(filePath)
				Expect(err).To(BeNil())
				Expect(pass).To(Equal("passphrase"))
			})
		})
	})
})
