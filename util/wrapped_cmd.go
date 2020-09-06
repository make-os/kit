package util

import (
	"io"
	"os/exec"
)

// Cmd provides an interface for exec.Cmd
type Cmd interface {
	Run() error
	Start() error
	Wait() error
	Output() ([]byte, error)
	CombinedOutput() ([]byte, error)
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	SetStderr(writer io.Writer)
	SetStdout(writer io.Writer)
	SetStdin(rdr io.Reader)
	ProcessWait() error
}

// WrappedCmd implements Cmd which exec.Cmd conforms to.
type WrappedCmd struct {
	cmd *exec.Cmd
}

// NewWrappedCmd creates an instance of WrappedCmd
func NewWrappedCmd(cmd *exec.Cmd) *WrappedCmd {
	return &WrappedCmd{cmd}
}

// WrappedCmd implements Cmd
func (w *WrappedCmd) Run() error {
	return w.cmd.Run()
}

// WrappedCmd implements Cmd
func (w *WrappedCmd) Start() error {
	return w.cmd.Start()
}

// WrappedCmd implements Cmd
func (w *WrappedCmd) Wait() error {
	return w.cmd.Wait()
}

// WrappedCmd implements Cmd
func (w *WrappedCmd) Output() ([]byte, error) {
	return w.cmd.Output()
}

// WrappedCmd implements Cmd
func (w *WrappedCmd) CombinedOutput() ([]byte, error) {
	return w.cmd.CombinedOutput()
}

// WrappedCmd implements Cmd
func (w *WrappedCmd) StdinPipe() (io.WriteCloser, error) {
	return w.cmd.StdinPipe()
}

// WrappedCmd implements Cmd
func (w *WrappedCmd) StdoutPipe() (io.ReadCloser, error) {
	return w.cmd.StdoutPipe()
}

// WrappedCmd implements Cmd
func (w *WrappedCmd) StderrPipe() (io.ReadCloser, error) {
	return w.cmd.StderrPipe()
}

func (w *WrappedCmd) SetStderr(writer io.Writer) {
	w.cmd.Stderr = writer
}

func (w *WrappedCmd) SetStdout(writer io.Writer) {
	w.cmd.Stdout = writer
}

func (w *WrappedCmd) SetStdin(rdr io.Reader) {
	w.cmd.Stdin = rdr
}

func (w *WrappedCmd) ProcessWait() error {
	_, err := w.cmd.Process.Wait()
	return err
}
