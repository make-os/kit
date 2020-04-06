package repo

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/types/state"
)

var _ = Describe("PolicyEnforcer", func() {
	Describe(".newPolicyEnforcer (test flattening)", func() {
		When("only distinct policies exist in groups", func() {
			It("should return all policies from all groups", func() {
				pe := newPolicyEnforcer([][]*state.Policy{
					{{Subject: "sub", Object: "obj", Action: "act"}},
					{{Subject: "sub2", Object: "obj2", Action: "act2"}},
				})
				Expect(pe.policies).To(HaveLen(2))
				Expect(pe.policies[0].level).To(Equal(0))
				Expect(pe.policies[1].level).To(Equal(1))
			})
		})

		When("duplicate policies exist in groups", func() {
			It("should resolve duplicates by selecting policies of lower levels", func() {
				pe := newPolicyEnforcer([][]*state.Policy{
					{{Subject: "sub", Object: "obj", Action: "act"}},
					{{Subject: "sub", Object: "obj", Action: "act"}},
				})
				Expect(pe.policies).To(HaveLen(1))
				Expect(pe.policies[0].level).To(Equal(0))
			})
		})
	})
})
