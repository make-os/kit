package testutil

type ReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
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
