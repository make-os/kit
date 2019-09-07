package mosdb_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func Testmosdb(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "mosdb Suite")
}
