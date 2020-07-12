package repocmd_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRepocmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Repocmd Suite")
}
