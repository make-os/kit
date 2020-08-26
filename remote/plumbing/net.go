package plumbing

import (
	"github.com/make-os/lobe/util/colorfmt"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/sideband"
)

func SidebandErr(msg string) []byte {
	return sideband.ErrorMessage.WithPayload([]byte(colorfmt.RedString(msg) + "\u001b[0m\n"))
}

func SidebandProgress(msg string) []byte {
	return sideband.ProgressMessage.WithPayload([]byte(colorfmt.GreenString(msg) + "\u001b[0m\n"))
}

func SidebandInfo(msg string) []byte {
	return sideband.ProgressMessage.WithPayload([]byte(colorfmt.WhiteBoldString(msg) + "\u001b[0m\n"))
}
