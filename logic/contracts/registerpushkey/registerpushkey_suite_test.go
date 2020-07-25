package registerpushkey_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRegisterpushkey(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registerpushkey Suite")
}
