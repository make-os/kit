package rand

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rand Suite")
}
