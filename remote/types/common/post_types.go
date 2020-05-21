package common

// IssueFields contains post body fields specific to issue post
type IssueFields struct {

	// Labels describes and classifies the post using keywords
	Labels *[]string `yaml:"labels,flow,omitempty" msgpack:"labels,omitempty"`

	// Assignees are the push keys assigned to the post
	Assignees *[]string `yaml:"assignees,flow,omitempty" msgpack:"assignees,omitempty"`
}

// MergeRequestFields contains post body fields specific to merge request posts
type MergeRequestFields struct {

	// BaseBranch is the destination branch name
	BaseBranch string `yaml:"base,omitempty" msgpack:"base,omitempty"`

	// BaseBranchHash is the destination branch current hash
	BaseBranchHash string `yaml:"baseHash,omitempty" msgpack:"baseHash,omitempty"`

	// TargetBranch is the name of the source branch
	TargetBranch string `yaml:"target,omitempty" msgpack:"target,omitempty"`

	// TargetBranchHash is the hash of the source branch
	TargetBranchHash string `yaml:"targetHash,omitempty" msgpack:"targetHash,omitempty"`
}
