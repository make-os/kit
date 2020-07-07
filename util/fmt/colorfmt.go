package fmt

import (
	"fmt"

	"github.com/fatih/color"
	"gitlab.com/makeos/mosdef/config"
)

// ColorFmt wraps fatih's Color providing a way to turn color off
// via a global variable set on app initialization
type ColorFmt struct {
	c *color.Color
}

// NewColor creates an instance of ColorFmt
func NewColor(attr ...color.Attribute) *ColorFmt {
	return &ColorFmt{c: color.New(attr...)}
}

func (c *ColorFmt) Sprint(a ...interface{}) string {
	if config.NoColorFormatting {
		return fmt.Sprint(a...)
	}
	return c.c.Sprint(a...)
}

func (c *ColorFmt) Sprintf(format string, a ...interface{}) string {
	if config.NoColorFormatting {
		return fmt.Sprintf(format, a...)
	}
	return c.c.Sprintf(format, a...)
}

func RedString(format string, a ...interface{}) string {
	return NewColor(color.FgRed).Sprintf(format, a...)
}

func YellowString(format string, a ...interface{}) string {
	return NewColor(color.FgYellow).Sprintf(format, a...)
}

func GreenString(format string, a ...interface{}) string {
	return NewColor(color.FgGreen).Sprintf(format, a...)
}

func CyanString(format string, a ...interface{}) string {
	return NewColor(color.FgCyan).Sprintf(format, a...)
}

func HiCyanString(format string, a ...interface{}) string {
	return NewColor(color.FgHiCyan).Sprintf(format, a...)
}

func HiBlackString(format string, a ...interface{}) string {
	return NewColor(color.FgHiBlack).Sprintf(format, a...)
}

func Red(format string, a ...interface{}) {
	fmt.Print(NewColor(color.FgRed).Sprintf(format, a...))
}

func Magenta(format string, a ...interface{}) {
	fmt.Print(NewColor(color.FgMagenta).Sprintf(format, a...))
}
