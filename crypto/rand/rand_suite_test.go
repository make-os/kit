package rand

import (
	"testing"

	"github.com/k0kubun/pp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rand Suite")
	rand := NewDRand()
	rand.Init()
	pp.Println(rand.Get(0))
}
