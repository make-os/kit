package ticket_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestTicket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ticket Suite")
}
