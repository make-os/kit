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
		// &state.Policy{Subject: "all", Object: "refs/heads", Action: PolicyActionMergeRequestWrite},

		// Everyone can create issues or merge request
		&state.Policy{Subject: "all", Object: issueRefPath, Action: PolicyActionIssueWrite},
		&state.Policy{Subject: "all", Object: mergeReqRefPath, Action: PolicyActionMergeRequestWrite},

		// Contributors default branch policies
		&state.Policy{Subject: "contrib", Object: "refs/heads", Action: PolicyActionWrite},             // can create branches
		&state.Policy{Subject: "contrib", Object: "refs/heads/master", Action: PolicyActionDenyDelete}, // cannot delete master branch
		&state.Policy{Subject: "contrib", Object: "refs/heads", Action: PolicyActionDelete},            // can delete any branches

		// Contributor default tag policies
		&state.Policy{Subject: "contrib", Object: "refs/tags", Action: PolicyActionWrite},   // can create tags
		&state.Policy{Subject: "contrib", Object: "refs/tags", Action: PolicyActionDelete},  // can delete any tags
		&state.Policy{Subject: "contrib", Object: "refs/notes", Action: PolicyActionWrite},  // can create notes
		&state.Policy{Subject: "contrib", Object: "refs/notes", Action: PolicyActionDelete}, // can delete any notes

		// Contributor default issue policies
		&state.Policy{Subject: "contrib", Object: issueRefPath, Action: PolicyActionIssueDelete}, // can delete issues
		&state.Policy{Subject: "contrib", Object: issueRefPath, Action: PolicyActionIssueUpdate}, // can update issue admin fields.

		// Creator default issue policies
		&state.Policy{Subject: "creator", Object: issueRefPath, Action: PolicyActionIssueDelete}, // can delete own issue
		&state.Policy{Subject: "creator", Object: issueRefPath, Action: PolicyActionIssueUpdate}, // can update own issue admin fields

		// Creator default merge request policies
		&state.Policy{Subject: "creator", Object: mergeReqRefPath, Action: PolicyActionMergeRequestDelete}, // can delete merge request
		&state.Policy{Subject: "creator", Object: mergeReqRefPath, Action: PolicyActionMergeRequestUpdate}, // can update own merge request admin fields

		// Contributor default merge request policies
		&state.Policy{Subject: "contrib", Object: mergeReqRefPath, Action: PolicyActionMergeRequestUpdate}, // can update any merge requests
		&state.Policy{Subject: "contrib", Object: mergeReqRefPath, Action: PolicyActionMergeRequestDelete}, // can delete any merge requests
	)
}
