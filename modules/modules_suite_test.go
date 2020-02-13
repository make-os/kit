package modules_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJsmodules(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "modules Suite")
}
