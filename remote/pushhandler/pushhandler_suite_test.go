package pushhandler_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPushhandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pushhandler Suite")
}
