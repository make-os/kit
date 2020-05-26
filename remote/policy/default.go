package policy

import (
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/types/state"
)

const (
	PolicyActionWrite              = "write"
	PolicyActionDelete             = "delete"
	PolicyActionDenyDelete         = "deny-delete"
	PolicyActionIssueWrite         = "issue-write"
	PolicyActionIssueDelete        = "issue-delete"
	PolicyActionIssueUpdate        = "issue-update"
	PolicyActionMergeRequestWrite  = "merge-write"
	PolicyActionMergeRequestUpdate = "merge-update"
	PolicyActionMergeRequestDelete = "merge-delete"
)

// AddDefaultPolicies adds default repo-level policies
func AddDefaultPolicies(config *state.RepoConfig) {
	issueRefPath := plumbing.MakeIssueReferencePath()
	mergeReqRefPath := plumbing.MakeMergeRequestReferencePath()
	config.Policies = append(
		config.Policies,
		&state.Policy{Subject: "all", Object: "refs/heads", Action: PolicyActionMergeRequestWrite},
		&state.Policy{Subject: "contrib", Object: "refs/heads", Action: PolicyActionWrite},
		&state.Policy{Subject: "contrib", Object: "refs/heads", Action: PolicyActionDelete},
		&state.Policy{Subject: "contrib", Object: "refs/heads/master", Action: PolicyActionDenyDelete},

		&state.Policy{Subject: "contrib", Object: "refs/tags", Action: PolicyActionWrite},
		&state.Policy{Subject: "contrib", Object: "refs/tags", Action: PolicyActionDelete},
		&state.Policy{Subject: "contrib", Object: "refs/notes", Action: PolicyActionWrite},
		&state.Policy{Subject: "contrib", Object: "refs/notes", Action: PolicyActionDelete},

		&state.Policy{Subject: "all", Object: issueRefPath, Action: PolicyActionIssueWrite},
		&state.Policy{Subject: "creator", Object: issueRefPath, Action: PolicyActionIssueDelete},
		&state.Policy{Subject: "contrib", Object: issueRefPath, Action: PolicyActionIssueDelete},
		&state.Policy{Subject: "creator", Object: issueRefPath, Action: PolicyActionIssueUpdate},
		&state.Policy{Subject: "contrib", Object: issueRefPath, Action: PolicyActionIssueUpdate},

		&state.Policy{Subject: "all", Object: mergeReqRefPath, Action: PolicyActionMergeRequestWrite},
		&state.Policy{Subject: "creator", Object: mergeReqRefPath, Action: PolicyActionMergeRequestDelete},
		&state.Policy{Subject: "contrib", Object: mergeReqRefPath, Action: PolicyActionMergeRequestDelete},
		&state.Policy{Subject: "creator", Object: mergeReqRefPath, Action: PolicyActionMergeRequestUpdate},
		&state.Policy{Subject: "contrib", Object: mergeReqRefPath, Action: PolicyActionMergeRequestUpdate},
	)
}
