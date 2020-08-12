package repocmd

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cast"
	restclient "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/cmd/common"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/util"
	gocfg "gopkg.in/src-d/go-git.v4/config"
)

type Remote struct {
	Name string
	URL  string
}

// ConfigArgs contains arguments for ConfigCmd.
type ConfigArgs struct {

	// Value is the transaction value
	Value *float64

	// Fee is the transaction fee to be paid by the signing key
	Fee *float64

	// Nonce is the next nonce of the signing account
	Nonce *uint64

	// CommitAmend amends and sign the last commit instead of creating new one.
	AmendCommit *bool

	// PrintOutForEval indicates that extra config for evaluation should be printed
	PrintOutForEval bool

	// SigningKey is the key that will be used to sign the transaction.
	SigningKey *string

	// SigningKeyPass is the passphrase for unlocking the signing key.
	SigningKeyPass *string

	// PushKey is the key that will be used sign push request
	PushKey *string

	// NoHook indicates that hooks should not be added
	NoHook bool

	// Remotes contain a list of git remotes to append to the repo.
	Remotes []Remote

	// RpcClient is the RPC client
	RPCClient client.Client

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore.
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce utils.NextNonceGetter

	// CreateRepo is a function for generating a transaction for creating a repository
	CreateRepo utils.RepoCreator

	Stdout io.Writer
}

// ConfigCmd configures a repository
func ConfigCmd(cfg *config.AppConfig, repo types.LocalRepo, args *ConfigArgs) error {

	rcfg, err := repo.Config()
	if err != nil {
		return err
	}

	// Set fee at `user.fee`
	if args.Fee != nil {
		rcfg.Raw.Section("user").SetOption("fee", cast.ToString(*args.Fee))
	}

	// Set value at `user.value`
	if args.Value != nil {
		rcfg.Raw.Section("user").SetOption("value", cast.ToString(*args.Value))
	}

	// Set nonce at `user.nonce`
	if args.Nonce != nil {
		rcfg.Raw.Section("user").SetOption("nonce", cast.ToString(*args.Nonce))
	}

	// Set signing key at `user.signingKey` only if args.PushKey is unset, otherwise use args.PushKey.
	if args.SigningKey != nil && (args.PushKey == nil || *args.PushKey == "") {
		rcfg.Raw.Section("user").SetOption("signingKey", *args.SigningKey)
	} else if args.PushKey != nil {
		rcfg.Raw.Section("user").SetOption("signingKey", *args.PushKey)
	}

	// Set signing key passphrase at `user.passphrase`
	if args.SigningKeyPass != nil {
		rcfg.Raw.Section("user").SetOption("passphrase", *args.SigningKeyPass)
	}

	// Set commit amendment `commit.amend`
	if args.AmendCommit != nil {
		rcfg.Raw.Section("commit").SetOption("amend", cast.ToString(*args.AmendCommit))
	}

	// Set lobe as `gpg.program`
	rcfg.Raw.Section("gpg").SetOption("program", config.ExecName)

	// Add user-defined remotes
	for _, remote := range args.Remotes {
		rcfg.Remotes[remote.Name] = &gocfg.RemoteConfig{Name: remote.Name, URLs: strings.Split(remote.URL, ",")}
	}

	// If no remote was set, add default remote pointing to the local remote.
	if len(rcfg.Remotes) == 0 {
		url := fmt.Sprintf("http://%s/r/%s", config.DefaultRemoteServerAddress, repo.GetName())
		rcfg.Remotes["origin"] = &gocfg.RemoteConfig{Name: "origin", URLs: []string{url}}
	}

	// Add hooks if allowed
	dotGitPath := filepath.Join(repo.GetPath(), ".git")
	if !args.NoHook {
		if err = addHooks(cfg.GetExecName(), dotGitPath); err != nil {
			return err
		}
	}

	// Set `sign.noUsername` to true.
	rcfg.Raw.Section("sign").SetOption("noUsername", "true")

	// Set auth hook to `core.askPass` and clear GIT_ASKPASS env var
	rcfg.Raw.Section("core").SetOption("askPass", filepath.Join(".git", "hooks", "askpass"))

	// Set the config
	if err = repo.SetConfig(rcfg); err != nil {
		return err
	}

	// Output more configurations that can only be evaluated in the parent environment.
	// If you are suspicious about the use of eval on the config command, this
	// is what is executed by eval:
	if args.PrintOutForEval && runtime.GOOS != "windows" {
		fmt.Fprint(os.Stdout, `unset GIT_ASKPASS`) // Unset so core.askPass is used instead.
	}

	return nil
}

// addHook adds hooks to git repo at the given path.
// If not already added the hook command already exist, it will not be re-added.
// If the hook file does not exist, create it and make it an executable on non-windows system.
func addHooks(appExecName string, path string) error {
	for _, hook := range []string{"pre-push", "askpass"} {
		cmd := fmt.Sprintf("%s repo hook $1", config.ExecName)

		if hook == "askpass" {
			cmd = fmt.Sprintf("%s repo hook --askpass $1", config.ExecName)
		}

		os.Mkdir(filepath.Join(path, "hooks"), 0700)
		prePushFile := filepath.Join(path, "hooks", hook)
		if !util.IsFileOk(prePushFile) {
			err := ioutil.WriteFile(prePushFile, []byte(fmt.Sprintf("#!/bin/sh\n%s", cmd)), 0644)
			if err != nil {
				return err
			}
			if runtime.GOOS == "windows" {
				continue
			}
			err = exec.Command("bash", "-c", "chmod +x "+prePushFile).Run()
			if err != nil {
				return err
			}
			continue
		}

		f, err := os.OpenFile(prePushFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		// Check if the hook command already exist in the file.
		// If it does, do not append again.
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			// ignore comment lines
			if line := scanner.Text(); line != "" && line[:1] == "#" {
				continue
			}
			if strings.Contains(scanner.Text(), appExecName+" repo hook") {
				goto end
			}
		}
		if scanner.Err() != nil {
			return err
		}
		_, err = f.WriteString("\n" + cmd)
		if err != nil {
			return err
		}
	end:
	}

	return nil
}
