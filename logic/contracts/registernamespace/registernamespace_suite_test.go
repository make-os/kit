package registernamespace_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAcquirenamespace(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Acquirenamespace Suite")
}
