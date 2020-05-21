package common

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

var (
	ErrBodyRequired  = fmt.Errorf("body is required")
	ErrTitleRequired = fmt.Errorf("title is required")
)

// WriteToPager spawns the specified page, passing the given content to it
func WriteToPager(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
	args := strings.Split(pagerCmd, " ")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	cmd.Stdin = content
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(stdOut, err.Error())
		fmt.Fprint(stdOut, content)
		return
	}
}

// pagerWriter describes a function for writing a specified content to a pager program
type PagerWriter func(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer)
