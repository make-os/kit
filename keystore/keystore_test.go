package keystore

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/make-os/kit/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// testPrompt2 will return response with index equal to count
// count is incremented each time the function is called.
func testPrompt2(count *int, responses []string) promptFunc {
	return func(prompt string, args ...interface{}) (string, error) {
		resp := responses[*count]
		*count++
		return resp, nil
	}
}

var _ = Describe("AccountMgr", func() {

	var oldStdout = os.Stdout
	path := filepath.Join("./", "test_cfg")
	accountPath := filepath.Join(path, config.KeystoreDirName)

	BeforeEach(func() {
		err := os.MkdirAll(accountPath, 0700)
		Expect(err).To(BeNil())
		_, w, _ := os.Pipe()
		os.Stdout = w
	})

	AfterEach(func() {
		os.Stdout = oldStdout
		err := os.RemoveAll(path)
		Expect(err).To(BeNil())
	})

	Describe(".hardenPassword", func() {
		It("should return [215, 59, 34, 12, 157, 105, 253, 31, 243, 128, 41, 222, 216, 93, "+
			"165, 77, 67, 179, 85, 192, 127, 47, 171, 121, 32, 117, 125, 119, 109, 243, 32, 95]", func() {
			bs := hardenPassword([]byte("abc"))
			Expect(bs).To(Equal([]byte{215, 59, 34, 12, 157, 105, 253, 31, 243, 128, 41, 222,
				216, 93, 165, 77, 67, 179, 85, 192, 127, 47, 171, 121, 32, 117, 125, 119, 109, 243, 32, 95}))
		})
	})

	Describe(".askForPassword", func() {
		am := New(accountPath)

		It("should return err = 'passphrases did not match'", func() {
			count := 0
			am.getPassword = testPrompt2(&count, []string{"passAbc", "passAb"})
			_, err := am.AskForPassword()
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("passphrases did not match"))
		})

		It("should return input even when no passphrase is provided the first time", func() {
			count := 0
			am.getPassword = testPrompt2(&count, []string{"", "passAb", "passAb"})
			passphrase, err := am.AskForPassword()
			Expect(err).To(BeNil())
			Expect(passphrase).To(Equal("passAb"))
		})
	})

	Describe(".askForPasswordOnce", func() {
		am := New(accountPath)
		am.SetOutput(ioutil.Discard)

		It("should return the first input received", func() {
			count := 0
			am.getPassword = testPrompt2(&count, []string{"", "", "passAb"})
			passphrase, _ := am.AskForPasswordOnce()
			Expect(passphrase).To(Equal("passAb"))
		})
	})
})
