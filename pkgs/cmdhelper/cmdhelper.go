package cmdhelper

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/thoas/go-funk"
)

const DefaultGroupName = ""
const globalGroupName = "Global"

type group struct {
	name  string
	flags []string
}

type CmdHelper struct {
	root  *cobra.Command
	group []*group
}

// NewCmdHelper creates an instance of CmdHelper
func NewCmdHelper(root *cobra.Command) *CmdHelper {
	return &CmdHelper{root: root, group: []*group{}}
}

// getFlagGroup returns the group of a flag
func (c *CmdHelper) getFlagGroup(flagName string) *group {
	for _, g := range c.group {
		if funk.ContainsString(g.flags, flagName) {
			return g
		}
	}
	return nil
}

func (c *CmdHelper) findGroup(name string) *group {
	for _, g := range c.group {
		if g.name == name {
			return g
		}
	}
	return nil
}

func (c *CmdHelper) addToGroup(groupName, flagName string) {
	grp := c.findGroup(groupName)
	if grp == nil {
		c.group = append(c.group, &group{name: groupName, flags: []string{flagName}})
		return
	}
	grp.flags = append(grp.flags, flagName)
	grp.flags = funk.UniqString(grp.flags)
}

// Grp register a flag to a group
func (c *CmdHelper) Grp(name, flagName string) *cobra.Command {
	c.addToGroup(name, flagName)
	return c.root
}

// Render generates a help message
func (c *CmdHelper) Render() *bytes.Buffer {

	out := bytes.NewBuffer(nil)

	// Command Description
	if c.root.Long != "" {
		out.WriteString(fmt.Sprintf("%s\n\n", c.root.Long))
	} else {
		out.WriteString(fmt.Sprintf("%s\n\n", c.root.Short))
	}

	// Command (and sub-command) Usage
	render(out, func(t table.Writer) {
		out.WriteString(fmt.Sprintf("Usage:\n"))
		t.SetColumnConfigs([]table.ColumnConfig{{Number: 1, WidthMin: 1}})
		var usages = []string{c.root.Use}
		for _, cmd := range c.root.Commands() {
			if !funk.ContainsString(usages, cmd.Use) {
				usages = append(usages, cmd.Use)
			}
		}
		for _, usage := range usages {
			var row []interface{}
			funk.ConvertSlice(strings.Split(usage, " "), &row)
			t.AppendRow(row)
		}
	})

	// Available Sub-commands
	render(out, func(t table.Writer) {
		out.WriteString(fmt.Sprintf("\nAvailable Commands:\n"))
		t.SetColumnConfigs([]table.ColumnConfig{{Number: 1, WidthMin: 8}})
		for _, cmd := range c.root.Commands() {
			var row = []interface{}{cmd.Name(), cmd.Short}
			t.AppendRow(row)
		}
	})

	// Add flags with no group to the default group
	c.root.Flags().VisitAll(func(flag *pflag.Flag) {
		if c.getFlagGroup(flag.Name) == nil {
			c.addToGroup(DefaultGroupName, flag.Name)
		}
	})

	c.root.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		if c.getFlagGroup(flag.Name) == nil {
			c.addToGroup(globalGroupName, flag.Name)
		}
	})

	// Flags
	for _, group := range c.group {
		title := group.name + " "
		if group.name == DefaultGroupName {
			title = ""
		}

		out.WriteString(fmt.Sprintf("\n%sFlags:\n", title))
		render(out, func(t table.Writer) {
			var flags []*pflag.Flag
			for _, flagName := range group.flags {
				fs := c.root.Flags()
				if group.name == globalGroupName {
					fs = c.root.PersistentFlags()
				}
				if flag := fs.Lookup(flagName); flag != nil {
					flags = append(flags, flag)
				}
			}

			t.SetColumnConfigs([]table.ColumnConfig{{Number: 1, WidthMin: 28}})
			for _, flag := range flags {
				var short = "   "
				if flag.Shorthand != "" {
					short = "-" + flag.Shorthand + ","
				}
				defTxt := ""
				if flag.DefValue != "" {
					defTxt = fmt.Sprintf("(default: \"%s\")", flag.DefValue)
				}
				rowTxt := fmt.Sprintf("%s --%s %s", short, flag.Name, flag.Value.Type())
				var row = []interface{}{rowTxt, fmt.Sprintf("%s %s", flag.Usage, defTxt)}
				t.AppendRow(row)
			}
		})
	}

	out.WriteString(fmt.Sprintf("\nUse \"%s --help\" for more information about a command.", c.root.Use))

	return out
}

func render(out *bytes.Buffer, f func(t table.Writer)) {
	t := table.NewWriter()
	t.SetOutputMirror(out)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false
	t.Style().Box.PaddingLeft = "  "
	f(t)
	t.Render()
}
