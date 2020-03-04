package rpc

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/util"
)

type testCase struct {
	params interface{}
	err    *rpc.Err
	result util.Map
	mocker func(tp testCase)
}

func TestRpc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rpc Suite")
}
