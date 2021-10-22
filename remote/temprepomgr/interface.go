package temprepomgr

type TempRepoManager interface {
	Add(path string) string
	GetPath(id string) string
	Remove(id string) error
}
