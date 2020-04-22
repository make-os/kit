package plumbing_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPlumbing(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plumbing Suite")
}
