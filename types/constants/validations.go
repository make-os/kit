package constants

import "fmt"

var (
	ErrProposalFeeNotExpected  = fmt.Errorf("proposal fee is not expected")
	ErrFullProposalFeeRequired = fmt.Errorf("full proposal fee is required")
)
