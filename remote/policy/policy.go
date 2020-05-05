package policy

import (
	"fmt"

	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/remote/plumbing"
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
// enforce is the enforcer function.
// pushKeyID is the push key of the pusher.
// isContributor indicates that the pusher is a contributor of the requested repository.
// reference is the target reference.
// action is the action requested by the user.
type PolicyChecker func(enforcer EnforcerFunc, pushKeyID string, isContributor bool, reference, action string) error

// CheckPolicy performs ACL checks to determine whether the given push key
// is permitted to perform the given action on the reference subject.
func CheckPolicy(enforcer EnforcerFunc, pushKeyID string, isContributor bool, reference, action string) error {

	rootDir := "refs/"
	if plumbing.IsIssueReference(reference) {
		rootDir = plumbing.MakeIssueReferencePath()
	} else if plumbing.IsBranch(reference) {
		rootDir = rootDir + "heads"
	} else if plumbing.IsTag(reference) {
		rootDir = rootDir + "tags"
	} else if plumbing.IsNote(reference) {
		rootDir = rootDir + "notes"
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
	res, lvl = enforcer("all", rootDir, action)
	if lvl > -1 && lvl <= highestLvl {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer("all", rootDir, negativeAct)
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
	res, lvl = enforcer(pushKeyID, rootDir, action)
	if lvl > -1 && lvl <= highestLvl {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer(pushKeyID, rootDir, negativeAct)
	if lvl > -1 && lvl <= highestLvl {
		allowed = !res
		highestLvl = lvl
	}

	// Skip to conclusion if pusher is not a contributor
	if !isContributor {
		goto conclude
	}

	// For only contributors
	// Check if the contributor can or cannot perform the action on the reference
	res, lvl = enforcer("contrib", reference, action)
	if lvl > -1 && lvl <= highestLvl {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer("contrib", reference, negativeAct)
	if lvl > -1 && lvl <= highestLvl {
		allowed = !res
		highestLvl = lvl
	}

	// Check if the contributor can or cannot perform the action on the reference directory
	res, lvl = enforcer("contrib", rootDir, action)
	if lvl > -1 && lvl <= highestLvl {
		allowed = res
		highestLvl = lvl
	}
	res, lvl = enforcer("contrib", rootDir, negativeAct)
	if lvl > -1 && lvl <= highestLvl {
		allowed = !res
		highestLvl = lvl
	}

conclude:
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

	// Find policies in the repo config-level policies
	// where the subject is "all", "contrib" or the pusher key ID
	// and also whose object points to a reference path or name
	for _, pol := range repoState.Config.Policies {
		if (funk.ContainsString([]string{"all", "contrib"}, pol.Subject) || pol.Subject == pushKeyID) &&
			plumbing.IsReference(pol.Object) {
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
