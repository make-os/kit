package refsync_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRefsync(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Refsync Suite")
}
