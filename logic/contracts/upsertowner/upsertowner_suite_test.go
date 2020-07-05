package upsertowner_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestUpsertowner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Upsertowner Suite")
}
