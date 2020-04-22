package policy

import (
	"fmt"

	"gitlab.com/makeos/mosdef/repo/plumbing"
	"gitlab.com/makeos/mosdef/types/state"
)

// EnforcerFunc describes a function used for checking policies.
// subject: The policy subject
// object: The policy object
// action: The policy action
type EnforcerFunc func(subject, object, action string) (bool, int)

// getPolicyEnforcer returns a policy enforcer function used for enforcing policies against a subject.
func GetPolicyEnforcer(policyGroup [][]*state.Policy) EnforcerFunc {
	enforcer := NewPolicyEnforcer(policyGroup)
	return func(subject, object, action string) (bool, int) {
		return enforcer.Enforce(subject, object, action)
	}
}

// policyChecker describes a function for enforcing repository policy
type PolicyChecker func(enforcer EnforcerFunc, pushKeyID, reference, action string) error

// CheckPolicy performs ACL checks to determine whether the given push key
// is permitted to perform the given action on the reference subject.
func CheckPolicy(enforcer EnforcerFunc, pushKeyID, reference, action string) error {

	dir := "refs/"
	if plumbing.IsBranch(reference) {
		dir = dir + "heads"
	} else if plumbing.IsTag(reference) {
		dir = dir + "tags"
	} else if plumbing.IsNote(reference) {
		dir = dir + "notes"
	} else {
		return fmt.Errorf("unknown reference (%s)", reference)
	}

	var negativeAct = "deny-" + action
	var allowed bool
	var highestLvl = 999 // Set default to a random, high number greater than all levels

	// Check if all push keys can or cannot perform the action to the target reference
	res, lvl := enforcer("all", reference, action)
	if lvl > -1 {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer("all", reference, negativeAct)
	if lvl > -1 && lvl <= highestLvl {
		allowed = !res
		highestLvl = lvl
	}

	// Check if all push keys can or cannot perform the action on the target reference directory
	res, lvl = enforcer("all", dir, action)
	if lvl > -1 && lvl <= highestLvl {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer("all", dir, negativeAct)
	if lvl > -1 && lvl <= highestLvl {
		allowed = !res
		highestLvl = lvl
	}

	// Check if the push key can or cannot perform the action on the reference
	res, lvl = enforcer(pushKeyID, reference, action)
	if lvl > -1 && lvl <= highestLvl {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer(pushKeyID, reference, negativeAct)
	if lvl > -1 && lvl <= highestLvl {
		allowed = !res
		highestLvl = lvl
	}

	// Check if the push key can or cannot perform the action on the reference directory
	res, lvl = enforcer(pushKeyID, dir, action)
	if lvl > -1 && lvl <= highestLvl {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer(pushKeyID, dir, negativeAct)
	if lvl > -1 && lvl <= highestLvl {
		allowed = !res
		highestLvl = lvl
	}

	if !allowed {
		return fmt.Errorf("reference (%s): not authorized to perform '%s' action", reference, action)
	}

	return nil
}

// MakePusherPolicyGroups creates a policy group contain the different category of policies
// a pusher can have. Currently, we have 3 policy levels namely, repo default policies,
// namespace contributor policies and repo contributor policies. Policies of lower slice
// indices take precedence than those at higher indices.
//
// Policy levels:
// - 0: Repo's contributor policy collection (highest precedence)
// - 1: Repo's namespace's contributor policy collection
// - 2: Repo's config policy collection
func MakePusherPolicyGroups(
	pushKeyID string,
	repoState *state.Repository,
	namespace *state.Namespace) [][]*state.Policy {

	// Gather the policies into groups
	var groups = make([][]*state.Policy, 3)

	// Find policies in the config-level policies where the subject is "all" or the pusher key ID
	// and also whose object is points to a reference path or name
	for _, pol := range repoState.Config.Policies {
		if (pol.Subject == "all" || pol.Subject == pushKeyID) && plumbing.IsReference(pol.Object) {
			groups[2] = append(groups[2], pol)
		}
	}

	// Add the pusher's namespace-level contributor policies
	if namespace != nil && namespace.Contributors.Has(pushKeyID) {
		for _, p := range namespace.Contributors[pushKeyID].Policies {
			groups[1] = append(groups[1], &state.Policy{Subject: pushKeyID, Object: p.Object, Action: p.Action})
		}
	}

	// Add the pusher's repo-level contributor policies
	if repoState.Contributors.Has(pushKeyID) {
		for _, p := range repoState.Contributors[pushKeyID].Policies {
			groups[0] = append(groups[0], &state.Policy{Subject: pushKeyID, Object: p.Object, Action: p.Action})
		}
	}

	return groups
}
