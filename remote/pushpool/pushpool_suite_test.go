package pushpool_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPushpool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pushpool Suite")
}
