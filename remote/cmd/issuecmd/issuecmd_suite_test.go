package issuecmd_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIssuecmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Issuecmd Suite")
}
