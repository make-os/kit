package announcer_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAnnouncer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Announcer Suite")
}
