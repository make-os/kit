package util

import (
	"github.com/AlecAivazis/survey/v2"
	// "github.com/c-bata/go-prompt"
	"github.com/howeyc/gopass"
)

type InputReaderArgs struct {
	Password bool
	Before   func()
	After    func(input string)
}

// InputReader describes a function for reading input from stdin
type InputReader func(title string, args *InputReaderArgs) string

// ReadInput starts a prompt to collect single line input
func ReadInput(title string, args *InputReaderArgs) string {
	if args.Before != nil {
		args.Before()
	}

	if args.Password {
		inp := readPasswordInput()
		if args.After != nil {
			args.After(inp)
		}
		return inp
	}

	var inp string
	survey.InputQuestionTemplate = title
	survey.AskOne(&survey.Input{Message: title}, &inp)

	if args.After != nil {
		args.After(inp)
	}

	return inp
}

// ConfirmInputReader describes a function for reading user confirmation
type ConfirmInputReader func(title string, def bool) bool

// ConfirmInput renders a confirm console input
func ConfirmInput(title string, def bool) bool {
	confirm := false
	survey.ConfirmQuestionTemplate = title + `{{if .Default}}(Y/n)>{{else}}(y/N)>{{" "}}{{- if .Answer}}{{.Answer}}{{"\n"}}{{end}}{{end}}`
	prompt := &survey.Confirm{Default: def}
	survey.AskOne(prompt, &confirm)
	return confirm
}

// readPasswordInput starts a prompt to collect single line password input
func readPasswordInput() string {
	password, err := gopass.GetPasswdMasked()
	if err != nil {
		panic(err)
	}
	return string(password[0:])
}
