package proposals_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestProposals(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Proposals Suite")
}
