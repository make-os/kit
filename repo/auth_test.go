package repo

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
)

var _ = FDescribe("Changes", func() {
	var err error
	var cfg *config.AppConfig

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".getPolicyEnforcer", func() {
		When("", func() {
			It("should", func() {
				// nsPolicies := []*state.RepoACLPolicy{{Subject: "gpg1", Object: "branch", Action: "push"}}
				// repoPolicies := []*state.RepoACLPolicy{{Subject: "gpg1", Object: "branch", Action: "delete"}}
				// getPolicyEnforcer(nsPolicies, repoPolicies)
			})
		})
	})
})
