package keepers_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestKeepers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keepers Suite")
}
