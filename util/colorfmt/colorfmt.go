package colorfmt

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/make-os/lobe/config"
)

func colorSprintf(format string, attr []color.Attribute, a ...interface{}) string {
	return NewColor(attr...).Sprintf(format, a...)
}

func colorSprint(format string, attr []color.Attribute) string {
	return NewColor(attr...).Sprint(format)
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
	return colorSprintf(format, []color.Attribute{color.FgRed}, a...)
}

func YellowString(fmt string, a ...interface{}) string {
	if len(a) == 0 {
		return colorSprint(fmt, []color.Attribute{color.FgYellow})
	}
	return colorSprintf(fmt, []color.Attribute{color.FgYellow}, a...)
}

func GreenString(format string, a ...interface{}) string {
	return colorSprintf(format, []color.Attribute{color.FgGreen}, a...)
}

func CyanString(format string, a ...interface{}) string {
	return colorSprintf(format, []color.Attribute{color.FgCyan}, a...)
}

func HiCyanString(format string, a ...interface{}) string {
	return colorSprintf(format, []color.Attribute{color.FgHiCyan}, a...)
}

func BoldString(format string, a ...interface{}) string {
	return colorSprintf(format, []color.Attribute{color.Bold}, a...)
}

func WhiteBoldString(format string, a ...interface{}) string {
	return colorSprintf(format, []color.Attribute{color.FgWhite, color.Bold}, a...)
}

func Red(format string, a ...interface{}) {
	fmt.Print(colorSprintf(format, []color.Attribute{color.FgRed}, a...))
}

func Magenta(format string, a ...interface{}) {
	fmt.Print(colorSprintf(format, []color.Attribute{color.FgMagenta}, a...))
}
