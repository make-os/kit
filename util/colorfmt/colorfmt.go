package colorfmt

import (
	"fmt"

	"github.com/fatih/color"
	"gitlab.com/makeos/mosdef/config"
)

func colorStr(format string, attr []color.Attribute, a ...interface{}) string {
	return NewColor(attr...).Sprintf(format, a...)
}

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
	return colorStr(format, []color.Attribute{color.FgRed}, a...)
}

func YellowString(fmt string, a ...interface{}) string {
	return colorStr(fmt, []color.Attribute{color.FgYellow}, a...)
}

func GreenString(format string, a ...interface{}) string {
	return colorStr(format, []color.Attribute{color.FgGreen}, a...)
}

func CyanString(format string, a ...interface{}) string {
	return colorStr(format, []color.Attribute{color.FgCyan}, a...)
}

func HiCyanString(format string, a ...interface{}) string {
	return colorStr(format, []color.Attribute{color.FgHiCyan}, a...)
}

func HiBlackString(format string, a ...interface{}) string {
	return colorStr(format, []color.Attribute{color.FgBlack}, a...)
}

func Red(format string, a ...interface{}) {
	fmt.Print(colorStr(format, []color.Attribute{color.FgRed}, a...))
}

func Magenta(format string, a ...interface{}) {
	fmt.Print(colorStr(format, []color.Attribute{color.FgMagenta}, a...))
}
