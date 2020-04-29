package testutil

type ReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}
