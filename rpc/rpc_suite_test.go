package rpc_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJsonrpc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RPC Suite")
}
