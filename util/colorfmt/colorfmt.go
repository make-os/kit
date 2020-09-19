package colorfmt

import (
	"fmt"

	"github.com/logrusorgru/aurora"
	"github.com/make-os/lobe/config"
)

type Attribute func(arg interface{}) aurora.Value

func colorSprintf(format string, attr []Attribute, a ...interface{}) string {
	return NewColor(attr...).Sprintf(format, a...)
}

func colorSprint(format string, attr []Attribute) string {
	return NewColor(attr...).Sprint(format)
}

// ColorFmt wraps fatih's Color providing a way to turn color off
// via a global variable set on app initialization
type ColorFmt struct {
	attrs []Attribute
}

// NewColor creates an instance of ColorFmt
func NewColor(attr ...Attribute) *ColorFmt {
	return &ColorFmt{attrs: attr}
}

func (c *ColorFmt) Sprint(format interface{}) string {
	if config.NoColorFormatting {
		return fmt.Sprint(format)
	}
	var v = aurora.White(format)
	for _, a := range c.attrs {
		v = a(v)
	}
	return aurora.Sprintf(v)
}

func (c *ColorFmt) Sprintf(format string, args ...interface{}) string {
	if config.NoColorFormatting {
		return fmt.Sprint(format, args)
	}
	var v = aurora.White(aurora.Sprintf(format, args...))
	for _, a := range c.attrs {
		v = a(v)
	}
	return v.String()
}

func RedString(format string, a ...interface{}) string {
	return colorSprintf(format, []Attribute{aurora.Red}, a...)
}

func YellowString(fmt string, a ...interface{}) string {
	return colorSprintf(fmt, []Attribute{aurora.Yellow}, a...)
}

func GreenString(format string, a ...interface{}) string {
	return colorSprintf(format, []Attribute{aurora.Green}, a...)
}

func CyanString(format string, a ...interface{}) string {
	return colorSprintf(format, []Attribute{aurora.Cyan}, a...)
}

func BrightCyanString(format string, a ...interface{}) string {
	return colorSprintf(format, []Attribute{aurora.BrightCyan}, a...)
}

func BoldString(format string, a ...interface{}) string {
	return colorSprintf(format, []Attribute{aurora.Bold}, a...)
}

func WhiteBoldString(format string, a ...interface{}) string {
	return colorSprintf(format, []Attribute{aurora.Bold, aurora.White}, a...)
}

func WhiteString(format string, a ...interface{}) string {
	return colorSprintf(format, []Attribute{aurora.White}, a...)
}

func Red(format string, a ...interface{}) {
	fmt.Print(colorSprintf(format, []Attribute{aurora.Red}, a...))
}

func Magenta(format string, a ...interface{}) {
	fmt.Print(colorSprintf(format, []Attribute{aurora.Magenta}, a...))
}
