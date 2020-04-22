package issues

import (
	"encoding/json"
	"fmt"
)

// MakeIssueBody creates an issue body using the specified fields
func MakeIssueBody(title, body, replyTo string, labels, assignees, fixers []string) string {
	args := ""
	str := "---\n%s---\n"

	if len(title) > 0 {
		args += fmt.Sprintf("title: %s\n", title)
	}
	if len(replyTo) > 0 {
		args += fmt.Sprintf("replyTo: %s\n", replyTo)
	}
	if len(labels) > 0 {
		labelsStr, _ := json.Marshal(labels)
		args += fmt.Sprintf("labels: %s\n", labelsStr)
	}
	if len(assignees) > 0 {
		assigneesStr, _ := json.Marshal(assignees)
		args += fmt.Sprintf("assignees: %s\n", assigneesStr)
	}
	if len(fixers) > 0 {
		fixersStr, _ := json.Marshal(fixers)
		args += fmt.Sprintf("fixers: %s\n", fixersStr)
	}

	return fmt.Sprintf(str, args) + body
}
