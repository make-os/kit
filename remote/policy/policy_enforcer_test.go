package policy_test

import (
	"github.com/make-os/lobe/remote/policy"
	"github.com/make-os/lobe/types/state"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PolicyEnforcerFunc", func() {
	Describe(".NewPolicyEnforcer (test flattening)", func() {
		When("only distinct policies exist in groups", func() {
			It("should return all policies from all groups", func() {
				pe := policy.NewPolicyEnforcer([][]*state.Policy{
					{{Subject: "sub", Object: "obj", Action: "act"}},
					{{Subject: "sub2", Object: "obj2", Action: "act2"}},
				})
				Expect(pe.GetPolicies()).To(HaveLen(2))
				Expect(pe.GetPolicies()[0].Level).To(Equal(0))
				Expect(pe.GetPolicies()[1].Level).To(Equal(1))
			})
		})

		When("duplicate policies exist in groups", func() {
			It("should resolve duplicates by selecting policies of lower levels", func() {
				pe := policy.NewPolicyEnforcer([][]*state.Policy{
					{{Subject: "sub", Object: "obj", Action: "act"}},
					{{Subject: "sub", Object: "obj", Action: "act"}},
				})
				Expect(pe.GetPolicies()).To(HaveLen(1))
				Expect(pe.GetPolicies()[0].Level).To(Equal(0))
			})
		})
	})
})
