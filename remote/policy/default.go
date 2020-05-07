package policy

import (
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/types/state"
)

// AddDefaultPolicies adds default repo-level policies
func AddDefaultPolicies(config *state.RepoConfig) {
	issueRefPath := plumbing.MakeIssueReferencePath()
	config.Policies = append(
		config.Policies,
		&state.Policy{Subject: "all", Object: "refs/heads", Action: "merge-write"},
		&state.Policy{Subject: "contrib", Object: "refs/heads", Action: "write"},
		&state.Policy{Subject: "contrib", Object: "refs/heads", Action: "delete"},
		&state.Policy{Subject: "contrib", Object: "refs/heads/master", Action: "deny-delete"},

		&state.Policy{Subject: "contrib", Object: "refs/tags", Action: "write"},
		&state.Policy{Subject: "contrib", Object: "refs/tags", Action: "delete"},
		&state.Policy{Subject: "contrib", Object: "refs/notes", Action: "write"},
		&state.Policy{Subject: "contrib", Object: "refs/notes", Action: "delete"},

		&state.Policy{Subject: "all", Object: issueRefPath, Action: "issue-write"},
		&state.Policy{Subject: "creator", Object: issueRefPath, Action: "issue-delete"},
		&state.Policy{Subject: "contrib", Object: issueRefPath, Action: "issue-delete"},
	)
}
