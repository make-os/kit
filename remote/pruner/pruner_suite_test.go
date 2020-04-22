package pruner_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPruner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pruner Suite")
}
