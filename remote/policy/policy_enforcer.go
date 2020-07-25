package policy

import (
	"github.com/themakeos/lobe/types/state"
)

type policyItem struct {
	policy *state.Policy
	level  int
}

type policyItems []*policyItem

func (p *policyItems) get(sub, obj, act string) *policyItem {
	for _, item := range *p {
		if item.policy.Subject == sub && item.policy.Object == obj && item.policy.Action == act {
			return item
		}
	}
	return nil
}

func (p *policyItems) add(policy *policyItem) {
	*p = append(*p, policy)
}

func (p *policyItems) replace(policyToReplace *policyItem, replacement *policyItem) {
	for i, policy := range *p {
		if *policy == *policyToReplace {
			(*p)[i] = replacement
		}
	}
}

// PolicyEnforcerFunc provides functionality for enforcing access level policies
// specifically for repositories.
type PolicyEnforcer struct {
	policies policyItems
}

// NewPolicyEnforcer creates an instance of PolicyEnforcerFunc; orderedPolicies are a slice
// of policies representing different groups and the group with the lower index have
// higher precedence.
func NewPolicyEnforcer(orderedPolicies [][]*state.Policy) *PolicyEnforcer {
	pol := &PolicyEnforcer{}
	pol.flatten(orderedPolicies)
	return pol
}

// flatten the given ordered policies
func (e *PolicyEnforcer) flatten(orderedPolicies [][]*state.Policy) {
	for level, policies := range orderedPolicies {
		for _, policy := range policies {
			existing := e.policies.get(policy.Subject, policy.Object, policy.Action)
			if existing == nil {
				e.policies.add(&policyItem{policy: policy, level: level})
			}
		}
	}
}

// Enforce determine whether a request is allowed or disallowed.
func (e *PolicyEnforcer) Enforce(sub, obj, act string) (allowed bool, level int) {
	found := e.policies.get(sub, obj, act)
	if found == nil {
		return false, -1
	}
	return true, found.level
}
