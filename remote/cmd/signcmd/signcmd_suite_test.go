package signcmd_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSigncmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Signcmd Suite")
}
