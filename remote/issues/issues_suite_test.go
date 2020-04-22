package issues_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIssues(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Issues Suite")
}
