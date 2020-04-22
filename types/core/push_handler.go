package core

import (
	"io"

	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
)

type PushHandler interface {
	HandleStream(packfile io.Reader, gitReceivePack io.WriteCloser) error
	HandleAuthorization(ur *packp.ReferenceUpdateRequest) error
	HandleReferences() error
	HandleUpdate() error
}
