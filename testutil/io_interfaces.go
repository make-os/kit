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

type WrapReadSeeker struct {
	Rdr io.Reader
}

func (w WrapReadSeeker) Read(p []byte) (n int, err error) {
	return w.Rdr.Read(p)
}

func (w WrapReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

type WrapReadSeekerCloser struct {
	Rdr      io.Reader
	CloseErr error
}

func (w WrapReadSeekerCloser) Close() error {
	return w.CloseErr
}

func (w WrapReadSeekerCloser) Read(p []byte) (n int, err error) {
	return w.Rdr.Read(p)
}

func (w WrapReadSeekerCloser) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

type Reader struct {
	Data []byte
	Err  error
}

func (r Reader) Read(p []byte) (n int, err error) {
	if r.Err == nil {
		r.Err = io.EOF
	}
	return copy(p, r.Data[:]), r.Err
}
