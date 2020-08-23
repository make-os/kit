package io

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/howeyc/gopass"
	"github.com/pkg/errors"
)

var ErrSrcTooLarge = fmt.Errorf("source is too large")

type InputReaderArgs struct {
	Password bool
	Before   func()
	After    func(input string)
}

// InputReader describes a function for reading input from stdin
type InputReader func(title string, args *InputReaderArgs) (string, error)

// ReadInput starts a prompt to collect single line input
func ReadInput(title string, args *InputReaderArgs) (string, error) {
	if args.Before != nil {
		args.Before()
	}

	survey.InputQuestionTemplate = title

	if args.Password {
		fmt.Print(survey.InputQuestionTemplate)
		inp, err := readPasswordInput()
		if err != nil {
			return "", err
		}
		if args.After != nil {
			args.After(inp)
		}
		return inp, nil
	}

	var inp string
	survey.AskOne(&survey.Input{Message: title}, &inp)

	if args.After != nil {
		args.After(inp)
	}

	return inp, nil
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
func readPasswordInput() (string, error) {
	password, err := gopass.GetPasswdMasked()
	if err != nil {
		return "", err
	}
	return string(password[0:]), nil
}

// LimitedReadToTmpFile copies n bytes from src into a temporary file.
// It returns ErrSizeTooLarge if the reader contains more than maxSize
// and EOF is src has contains less bytes than maxSize.
// The caller is responsible for closing the returned reader.
func LimitedReadToTmpFile(src io.Reader, limit int64) (ReadSeekerCloser, error) {
	w, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create tmp file")
	}

	// Read max size. Return nil on EOF.
	_, err = io.CopyN(w, src, limit)
	if err != nil {
		if err == io.EOF {
			w.Seek(0, 0)
			return w, nil
		}
		return nil, err
	}
	w.Seek(0, 0)

	// Read additional to determine if there are more bytes after maxSize.
	n2, err := src.Read([]byte{0})
	if err != nil {
		if err == io.EOF {
			return w, nil
		}
		return nil, err
	} else if n2 > 0 {
		return nil, ErrSrcTooLarge
	}

	return w, nil
}

type ReadSeekerCloser interface {
	io.ReadSeeker
	io.Closer
}
