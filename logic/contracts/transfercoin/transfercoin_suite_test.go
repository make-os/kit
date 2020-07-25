package transfercoin_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestTransfercoin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Transfercoin Suite")
}
