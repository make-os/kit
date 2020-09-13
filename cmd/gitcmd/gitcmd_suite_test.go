package gitcmd_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGitcmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GitCmd Suite")
}
