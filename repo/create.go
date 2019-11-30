package repo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
)

// CreateRepository creates a bare git repository
func (m *Manager) CreateRepository(name string) error {

	// Create the path if it does not exist
	path := filepath.Join(m.rootDir, name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("a repository with name (%s) already exist", name)
	}

	// Create the repository
	_, err := git.PlainInit(path, true)
	if err != nil {
		return errors.Wrap(err, "failed to create repo")
	}

	// Set config options
	options := [][]string{
		{"gc.auto", "0"},
	}
	for _, opt := range options {
		_, err = execGitCmd(m.gitBinPath, path, append([]string{"config"}, opt...)...)
		if err != nil {
			return errors.Wrap(err, "failed to set config")
		}
	}

	return err
}

// HasRepository returns true if a valid repository exist
// for the given name
func (m *Manager) HasRepository(name string) bool {

	path := filepath.Join(m.rootDir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}

	if _, err := git.PlainOpen(path); err != nil {
		return false
	}

	return true
}
