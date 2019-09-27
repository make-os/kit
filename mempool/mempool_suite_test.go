package mempool
import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMempool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mempool Suite")
}
