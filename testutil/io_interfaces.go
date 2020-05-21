package testutil

import "io"

type ReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}

// FileReader provides a minimal interface for Stdout.
type FileReader interface {
	io.Reader
	io.Closer
	Fd() uintptr
}

// WrapReadCloser implements ReadCloser
type WrapReadCloser struct {
	Buf []byte
	Err error
}

func (w WrapReadCloser) Read(p []byte) (n int, err error) {
	copy(p, w.Buf[:])
	return len(w.Buf), w.Err
}

func (w WrapReadCloser) Close() error {
	return nil
}

// FileWriter provides a minimal interface for Stdin.
type FileWriter interface {
	io.Writer
	Fd() uintptr
}
