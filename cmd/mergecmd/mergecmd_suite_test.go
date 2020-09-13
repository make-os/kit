package mergecmd_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMergecmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MergeCmd Suite")
}
