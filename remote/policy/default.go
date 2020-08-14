package policy

import (
	"github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/types/state"
)

const (
	PolicyActionWrite      = "write"
	PolicyActionDelete     = "delete"
	PolicyActionUpdate     = "update"
	PolicyActionDenyDelete = "deny-delete"
)

// AddDefaultPolicies adds default repo-level policies
func AddDefaultPolicies(config *state.RepoConfig) {
	issueRefPath := plumbing.MakeIssueReferencePath()
	mergeReqRefPath := plumbing.MakeMergeRequestReferencePath()
	config.Policies = append(
		config.Policies,

		// Everyone can create issues or merge request
		&state.Policy{Subject: "all", Object: issueRefPath, Action: PolicyActionWrite},
		&state.Policy{Subject: "all", Object: mergeReqRefPath, Action: PolicyActionWrite},

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
		&state.Policy{Subject: "contrib", Object: issueRefPath, Action: PolicyActionDelete}, // can delete issues
		&state.Policy{Subject: "contrib", Object: issueRefPath, Action: PolicyActionUpdate}, // can update issue admin fields.

		// Creator default issue policies
		&state.Policy{Subject: "creator", Object: issueRefPath, Action: PolicyActionDelete}, // can delete own issue
		&state.Policy{Subject: "creator", Object: issueRefPath, Action: PolicyActionUpdate}, // can update own issue admin fields

		// Creator default merge request policies
		&state.Policy{Subject: "creator", Object: mergeReqRefPath, Action: PolicyActionDelete}, // can delete merge request
		&state.Policy{Subject: "creator", Object: mergeReqRefPath, Action: PolicyActionUpdate}, // can update own merge request admin fields

		// Contributor default merge request policies
		&state.Policy{Subject: "contrib", Object: mergeReqRefPath, Action: PolicyActionUpdate}, // can update any merge requests
		&state.Policy{Subject: "contrib", Object: mergeReqRefPath, Action: PolicyActionDelete}, // can delete any merge requests
	)
}
