package rest

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type TestCase struct {
	params     map[string]string
	body       string
	statusCode int
	mocker     func(tc *TestCase)
}

func TestRest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rest Suite")
}
