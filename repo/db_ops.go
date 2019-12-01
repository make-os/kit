package repo

type DBOperations interface {
	GetCache() *DBCache
}

// DBOps is a local database for storing repo-specific data
type DBOps struct {
	dbCache  *DBCache
	repoName string
}

// NewDBOps creates an instance of DBOps
func NewDBOps(dbCache *DBCache, repoName string) *DBOps {
	return &DBOps{dbCache: dbCache, repoName: repoName}
}

// GetCache Implements types.RepoDBOps
func (c *DBOps) GetCache() *DBCache {
	return c.dbCache
}
