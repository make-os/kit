package keystore

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAccount(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keystore Suite")
}
