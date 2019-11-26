package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
)

var GitEnv = os.Environ()

func execGit(workDir string, arg ...string) []byte {
	cmd := exec.Command(gitPath, arg...)
	cmd.Dir = workDir
	cmd.Env = GitEnv
	bz, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(bz))
		panic(err)
	}
	return bz
}

func appendToFile(path, file string, data string) {
	script.Echo(data).AppendFile(filepath.Join(path, file))
}

func execGitCommit(path, msg string) []byte {
	execGit(path, "add", ".")
	return execGit(path, "commit", "-m", msg)
}

func execGitSignedCommit(path, msg, keyID string) []byte {
	execGit(path, "add", ".")
	return execGit(path, "commit", "-m", msg, "-S"+keyID)
}

func appendCommit(path, file, fileData, commitMsg string) {
	appendToFile(path, file, fileData)
	execGitCommit(path, commitMsg)
}

func appendDirAndCommitFile(path, targetDir, file, fileData, commitMsg string) {
	execAnyCmd(path, "mkdir", targetDir)
	appendToFile(path, filepath.Join(targetDir, file), fileData)
	execGitCommit(path, commitMsg)
}

func appendSignedCommit(path, file, fileData, commitMsg, keyID string) {
	appendToFile(path, file, fileData)
	execGitSignedCommit(path, commitMsg, keyID)
}

func createCommitAndAnnotatedTag(path, file, fileData, commitMsg, tagName string) {
	appendToFile(path, file, fileData)
	execGitCommit(path, commitMsg)
	execGit(path, "tag", "-a", tagName, "-m", commitMsg, "-f")
}

func createCommitAndSignedAnnotatedTag(path, file, fileData, commitMsg, tagName, keyID string) {
	appendToFile(path, file, fileData)
	execGitCommit(path, commitMsg)
	execGit(path, "tag", "-a", tagName, "-m", commitMsg, "-f", "-u"+keyID)
}

func createSignedCommitAndSignedAnnotatedTag(path, file, fileData, commitMsg, tagName, keyID string) {
	appendToFile(path, file, fileData)
	execGitSignedCommit(path, commitMsg, keyID)
	execGit(path, "tag", "-a", tagName, "-m", commitMsg, "-f", "-u"+keyID)
}

func createCommitAndLightWeightTag(path, file, fileData, commitMsg, tagName string) {
	appendToFile(path, file, fileData)
	execGitCommit(path, commitMsg)
	execGit(path, "tag", tagName, "-f")
}

func createSignedCommitAndLightWeightTag(path, file, fileData, commitMsg, tagName, keyID string) {
	appendToFile(path, file, fileData)
	execGitSignedCommit(path, commitMsg, keyID)
	execGit(path, "tag", tagName, "-f")
}

func createCommitAndNote(path, file, fileData, commitMsg, noteName string) {
	appendToFile(path, file, fileData)
	execGitCommit(path, commitMsg)
	execGit(path, "notes", "--ref", noteName, "add", "-m", commitMsg, "-f")
}

func createBlob(path, content string) string {
	hash, err := script.Echo("").ExecInDir(`git hash-object -w --stdin`, path).String()
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(hash)
}

func createNoteEntry(path, noteName, msg string) string {
	hash, err := script.Echo("").ExecInDir(`git hash-object -w --stdin`, path).String()
	if err != nil {
		panic(err)
	}
	execGit(path, "notes", "--ref", noteName, "add", "-m", msg, "-f", strings.TrimSpace(hash))
	return strings.TrimSpace(hash)
}

func deleteTag(path, name string) {
	execGit(path, "tag", "-d", name)
}

func deleteNote(path, name string) {
	execGit(path, "update-ref", "-d", name)
}

func scriptFile(path, file string) *script.Pipe {
	return script.File(filepath.Join(path, file))
}

func createCheckoutBranch(path, branch string) {
	execGit(path, "checkout", "-b", branch)
}

func execAnyCmd(workDir, name string, arg ...string) []byte {
	cmd := exec.Command(name, arg...)
	cmd.Dir = workDir
	bz, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	return bz
}

func getRecentCommitHash(path, ref string) string {
	return strings.TrimSpace(string(execGit(path, "--no-pager", "log", ref, "-1", "--format=%H")))
}
