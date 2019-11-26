package repo

// DBOps is a local database for storing repo-specific data
type DBOps struct {
	dbCache  *DBCache
	repoName string
}

// NewDBOps creates an instance of DBOps
func NewDBOps(dbCache *DBCache, repoName string) *DBOps {
	return &DBOps{dbCache: dbCache, repoName: repoName}
}
