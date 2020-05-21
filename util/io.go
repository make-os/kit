package util

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/c-bata/go-prompt"
	"github.com/howeyc/gopass"
)

type InputReaderArgs struct {
	Password  bool
	Multiline bool
}

// InputReader describes a function for reading input from stdin
type InputReader func(title string, args *InputReaderArgs) string

// ReadInput starts a prompt to collect single line input
func ReadInput(title string, args *InputReaderArgs) string {
	if args.Password {
		return readPasswordInput(title)
	} else if args.Multiline {
		return readInputMultiline(title)
	}
	fmt.Print(title)
	return prompt.Input("", func(prompt.Document) []prompt.Suggest { return nil })
}

// readPasswordInput starts a prompt to collect single line password input
func readPasswordInput(title string) string {
	fmt.Print(title)
	password, err := gopass.GetPasswdMasked()
	if err != nil {
		panic(err)
	}
	return string(password[0:])
}

// readInputMultiline starts a prompt to collect multiline input
func readInputMultiline(title string) string {
	var str string
	survey.MultilineQuestionTemplate = title
	survey.AskOne(&survey.Multiline{}, &str)
	return str
}
