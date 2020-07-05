package createrepo_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCreaterepo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Createrepo Suite")
}
