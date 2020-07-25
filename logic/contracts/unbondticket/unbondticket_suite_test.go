package unbondticket_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestUnbondticket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Unbondticket Suite")
}
