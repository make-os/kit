package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
)

var GitEnv = os.Environ()
var gitPath = "/usr/bin/git"

func ExecGit(workDir string, args ...string) []byte {
	cmd := exec.Command(gitPath, args...)
	cmd.Dir = workDir
	cmd.Env = GitEnv
	bz, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}
	return bz
}

func AppendToFile(path, file string, data string) {
	_, _ = script.Echo(data).AppendFile(filepath.Join(path, file))
}

func ExecGitCommit(path, msg string) []byte {
	ExecGit(path, "add", ".")
	return ExecGit(path, "commit", "-m", msg)
}

func AppendCommit(path, file, fileData, commitMsg string) {
	AppendToFile(path, file, fileData)
	ExecGitCommit(path, commitMsg)
}

func AppendDirAndCommitFile(path, targetDir, file, fileData, commitMsg string) {
	ExecAnyCmd(path, "mkdir", targetDir)
	AppendToFile(path, filepath.Join(targetDir, file), fileData)
	ExecGitCommit(path, commitMsg)
}

func CreateCommitAndAnnotatedTag(path, file, fileData, commitMsg, tagName string) {
	AppendToFile(path, file, fileData)
	ExecGitCommit(path, commitMsg)
	ExecGit(path, "tag", "-a", tagName, "-m", commitMsg, "-f")
}

func CreateCommitAndLightWeightTag(path, file, fileData, commitMsg, tagName string) {
	AppendToFile(path, file, fileData)
	ExecGitCommit(path, commitMsg)
	ExecGit(path, "tag", tagName, "-f")
}

func CreateCommitAndNote(path, file, fileData, commitMsg, noteName string) {
	AppendToFile(path, file, fileData)
	ExecGitCommit(path, commitMsg)
	ExecGit(path, "notes", "--ref", noteName, "add", "-m", commitMsg, "-f")
}

func CreateNote(path, msg, noteName string) {
	ExecGit(path, "notes", "--ref", noteName, "add", "-m", msg, "-f")
}

func CreateBlob(path, content string) string {
	hash, err := script.Echo(content).ExecInDir(`git hash-object -w --stdin`, path).String()
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(hash)
}

func CreateNoteEntry(path, noteName, msg string) string {
	hash, err := script.Echo("").ExecInDir(`git hash-object -w --stdin`, path).String()
	if err != nil {
		panic(err)
	}
	ExecGit(path, "notes", "--ref", noteName, "add", "-m", msg, "-f", strings.TrimSpace(hash))
	return strings.TrimSpace(hash)
}

func DeleteTag(path, name string) {
	ExecGit(path, "tag", "-d", name)
}

func DeleteNote(path, name string) {
	ExecGit(path, "update-ref", "-d", name)
}

func ScriptFile(path, file string) *script.Pipe {
	return script.File(filepath.Join(path, file))
}

func CreateCheckoutBranch(path, branch string) {
	ExecGit(path, "checkout", "-b", branch)
}

func CheckoutBranch(path, branch string) {
	ExecGit(path, "checkout", branch)
}

func ForceMergeOurs(path, targetBranch string) {
	ExecGit(path, "merge", targetBranch, "-X", "ours")
}

func ExecAnyCmd(workDir, name string, args ...string) []byte {
	cmd := exec.Command(name, args...)
	cmd.Dir = workDir
	bz, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	return bz
}

func GetRecentCommitHash(path, ref string) string {
	return strings.TrimSpace(string(ExecGit(path, "--no-pager", "log", ref, "-1", "--format=%H")))
}
