package repocmd

// CreateArgs contains arguments for CreateCmd.
type CreateArgs struct {
	// Name is the name of the repository
	Name string
}

// CreateCmd creates a repository
func CreateCmd(args *CreateArgs) error {
	return nil
}
