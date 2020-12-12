package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// RejectFlagCombo rejects unwanted flag combinations
func RejectFlagCombo(cmd *cobra.Command, flags ...string) error {
	var found []string
	for _, f := range flags {
		if len(found) > 0 && cmd.Flags().Changed(f) {
			str := "--" + f
			if fShort := cmd.Flags().Lookup(f).Shorthand; fShort != "" {
				str += "|-" + fShort
			}
			found = append(found, str)
			return fmt.Errorf("flags %s can't be used together", strings.Join(found, ", "))
		}
		if cmd.Flags().Changed(f) {
			str := "--" + f
			if fShort := cmd.Flags().Lookup(f).Shorthand; fShort != "" {
				str += "|-" + fShort
			}
			found = append(found, str)
		}
	}
	return nil
}
