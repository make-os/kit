package plumbing

import (
	"github.com/make-os/lobe/util/colorfmt"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/sideband"
)

// SidebandErr creates a sideband error message
func SidebandErr(msg string) []byte {
	return sideband.ErrorMessage.WithPayload([]byte(colorfmt.RedString(msg)))
}

// SidebandProgressln creates a sideband progress message with a newline prefix
func SidebandProgressln(msg string) []byte {
	return sideband.ProgressMessage.WithPayload([]byte(colorfmt.GreenString(msg) + "\n"))
}

// SidebandInfoln creates a sideband progress info message with a newline prefix
func SidebandInfoln(msg string) []byte {
	return sideband.ProgressMessage.WithPayload([]byte(colorfmt.WhiteString(msg) + "\n"))
}

// SidebandYellowln is like SidebandInfoln but with yellow colored text
func SidebandYellowln(msg string) []byte {
	return sideband.ProgressMessage.WithPayload([]byte(colorfmt.YellowStringf(msg) + "\n"))
}
