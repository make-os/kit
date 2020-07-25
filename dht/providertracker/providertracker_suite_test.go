package providertracker_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestProvidertracker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Providertracker Suite")
}
