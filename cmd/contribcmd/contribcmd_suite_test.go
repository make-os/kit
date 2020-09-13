package contribcmd_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestContribCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RepoCmd Suite")
}
